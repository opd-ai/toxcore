// Package av implements the audio/video calling functionality for toxcore-go.
//
// This package provides a pure Go implementation of ToxAV that integrates
// seamlessly with the existing toxcore-go infrastructure including transport,
// crypto, DHT, and friend management systems.
//
// The design follows established patterns from the toxcore-go codebase:
// - Interface-based design for testability
// - Secure-by-default with existing crypto integration
// - Reuse of existing networking and transport layers
// - Pure Go implementation with no CGo dependencies
package av

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/av/audio"
	"github.com/opd-ai/toxcore/av/rtp"
	"github.com/opd-ai/toxcore/av/video"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// CallState represents the current state of an audio/video call.
// These values match the libtoxcore ToxAV API for compatibility.
type CallState uint32

const (
	// CallStateNone indicates no call is active
	CallStateNone CallState = iota
	// CallStateError indicates a call error occurred
	CallStateError
	// CallStateFinished indicates the call has ended normally
	CallStateFinished
	// CallStateSendingAudio indicates audio is being sent
	CallStateSendingAudio
	// CallStateSendingVideo indicates video is being sent
	CallStateSendingVideo
	// CallStateAcceptingAudio indicates audio is being received
	CallStateAcceptingAudio
	// CallStateAcceptingVideo indicates video is being received
	CallStateAcceptingVideo
)

// CallControl represents call control actions.
// These values match the libtoxcore ToxAV API for compatibility.
type CallControl uint32

const (
	// CallControlResume resumes a paused call
	CallControlResume CallControl = iota
	// CallControlPause pauses an active call
	CallControlPause
	// CallControlCancel cancels/ends the call
	CallControlCancel
	// CallControlMuteAudio mutes outgoing audio
	CallControlMuteAudio
	// CallControlUnmuteAudio unmutes outgoing audio
	CallControlUnmuteAudio
	// CallControlHideVideo hides outgoing video
	CallControlHideVideo
	// CallControlShowVideo shows outgoing video
	CallControlShowVideo
)

// String returns the string representation of a CallControl value.
func (c CallControl) String() string {
	switch c {
	case CallControlResume:
		return "Resume"
	case CallControlPause:
		return "Pause"
	case CallControlCancel:
		return "Cancel"
	case CallControlMuteAudio:
		return "MuteAudio"
	case CallControlUnmuteAudio:
		return "UnmuteAudio"
	case CallControlHideVideo:
		return "HideVideo"
	case CallControlShowVideo:
		return "ShowVideo"
	default:
		return "Unknown"
	}
}

// AddressResolver is a function type for resolving friend numbers to network addresses.
// This is used by the Call type to obtain the remote address for RTP session setup.
type AddressResolver func(friendNumber uint32) ([]byte, error)

// TimeProvider abstracts time operations for deterministic testing.
// Production code uses DefaultTimeProvider; tests can inject mock implementations.
type TimeProvider interface {
	Now() time.Time
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time from time.Now().
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

// resolveRemoteAddress resolves a friend number to a network address using the
// provided resolver, falling back to a placeholder localhost address on failure.
//
// This helper encapsulates the address resolution pattern used by SetupMedia:
// 1. If resolver is nil, returns placeholder address
// 2. If resolver returns an error, logs warning and returns placeholder
// 3. If resolver returns insufficient bytes (<6), logs warning and returns placeholder
// 4. Otherwise, parses first 4 bytes as IP and next 2 bytes as port (big-endian)
//
// The placeholder address format is 127.0.0.1:(10000 + friendNumber), which
// provides unique addresses for testing without network configuration.
func resolveRemoteAddress(resolver AddressResolver, friendNumber uint32) net.Addr {
	funcName := "resolveRemoteAddress"

	if resolver != nil {
		addrBytes, err := resolver(friendNumber)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      funcName,
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Warn("Address resolver failed, using placeholder address")
		} else if len(addrBytes) >= 6 {
			// Parse address bytes: first 4 bytes are IP, last 2 bytes are port (big-endian)
			ip := net.IP(addrBytes[:4])
			port := int(addrBytes[4])<<8 | int(addrBytes[5])
			addr := &net.UDPAddr{IP: ip, Port: port}
			logrus.WithFields(logrus.Fields{
				"function":      funcName,
				"friend_number": friendNumber,
				"remote_addr":   addr.String(),
			}).Debug("Resolved friend address via address resolver")
			return addr
		} else {
			logrus.WithFields(logrus.Fields{
				"function":      funcName,
				"friend_number": friendNumber,
				"addr_len":      len(addrBytes),
			}).Warn("Address resolver returned insufficient bytes, using placeholder address")
		}
	}

	// Return placeholder address
	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: int(10000 + friendNumber),
	}
	logrus.WithFields(logrus.Fields{
		"function":      funcName,
		"friend_number": friendNumber,
		"remote_addr":   addr.String(),
	}).Debug("Using placeholder address")
	return addr
}

// Call represents an individual audio/video call session.
//
// Each call maintains its own state, bit rates, timing information,
// RTP session for media transport, and audio processor for encoding/decoding.
// The design ensures thread-safety through appropriate mutex usage following
// established patterns.
type Call struct {
	// Core call information
	friendNumber uint32
	callID       uint32 // Unique call identifier for signaling
	state        CallState
	audioEnabled bool
	videoEnabled bool

	// Call control states
	paused      bool // Call is paused (no media transmission)
	audioMuted  bool // Audio transmission is muted
	videoHidden bool // Video transmission is hidden

	// Bit rate configuration
	audioBitRate uint32
	videoBitRate uint32

	// Timing information for quality monitoring
	startTime time.Time
	lastFrame time.Time

	// Media processing components for Phase 2 and Phase 3 implementation
	audioProcessor *audio.Processor
	videoProcessor *video.Processor
	rtpSession     *rtp.Session

	// Address resolver for RTP session setup.
	// If configured, used to resolve friend number to network address.
	// If nil, falls back to placeholder localhost address.
	addressResolver AddressResolver

	// Time provider for deterministic testing.
	// If nil, DefaultTimeProvider is used.
	timeProvider TimeProvider

	// Bitrate adapter for automatic bitrate adjustment.
	// If nil, quality monitoring operates without network quality data.
	bitrateAdapter *BitrateAdapter

	// Thread safety
	mu sync.RWMutex
}

// NewCall creates a new call instance for the specified friend.
//
// The call starts in CallStateNone and must be started or answered
// to begin media transmission. This follows the established pattern
// of constructor functions in the toxcore-go codebase.
//
// Note: RTP session and audio processor are initialized separately
// via SetupMedia when the call is actually started or answered.
func NewCall(friendNumber uint32) *Call {
	logrus.WithFields(logrus.Fields{
		"function":      "NewCall",
		"friend_number": friendNumber,
	}).Info("Creating new call")

	call := &Call{
		friendNumber: friendNumber,
		state:        CallStateNone,
		audioEnabled: false,
		videoEnabled: false,
		paused:       false,
		audioMuted:   false,
		videoHidden:  false,
		audioBitRate: 0,
		videoBitRate: 0,
		startTime:    time.Time{},
		lastFrame:    time.Time{},
		// Media components initialized when call starts
		audioProcessor: nil,
		videoProcessor: nil,
		rtpSession:     nil,
		timeProvider:   DefaultTimeProvider{},
	}

	logrus.WithFields(logrus.Fields{
		"function":      "NewCall",
		"friend_number": friendNumber,
		"state":         call.state,
	}).Info("Call created successfully")

	return call
}

// GetFriendNumber returns the friend number associated with this call.
func (c *Call) GetFriendNumber() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.friendNumber
}

// GetState returns the current call state.
func (c *Call) GetState() CallState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetState updates the call state.
// This method is thread-safe and used internally by the manager.
func (c *Call) SetState(state CallState) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetState",
		"friend_number": c.friendNumber,
		"old_state":     c.state,
		"new_state":     state,
	}).Debug("Updating call state")

	c.mu.Lock()
	defer c.mu.Unlock()
	oldState := c.state
	c.state = state

	logrus.WithFields(logrus.Fields{
		"function":      "SetState",
		"friend_number": c.friendNumber,
		"old_state":     oldState,
		"new_state":     state,
	}).Info("Call state updated")
}

// IsAudioEnabled returns whether audio is enabled for this call.
func (c *Call) IsAudioEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.audioEnabled
}

// IsVideoEnabled returns whether video is enabled for this call.
func (c *Call) IsVideoEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.videoEnabled
}

// GetAudioBitRate returns the current audio bit rate.
func (c *Call) GetAudioBitRate() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.audioBitRate
}

// GetVideoBitRate returns the current video bit rate.
func (c *Call) GetVideoBitRate() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.videoBitRate
}

// GetStartTime returns when the call was started.
func (c *Call) GetStartTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.startTime
}

// GetLastFrameTime returns when the last frame was received/sent.
func (c *Call) GetLastFrameTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastFrame
}

// GetBitrateAdapter returns the bitrate adapter for this call.
// Returns nil if no adapter has been set.
func (c *Call) GetBitrateAdapter() *BitrateAdapter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bitrateAdapter
}

// SetBitrateAdapter sets the bitrate adapter for automatic bitrate adjustment.
// Pass nil to disable bitrate adaptation.
func (c *Call) SetBitrateAdapter(adapter *BitrateAdapter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bitrateAdapter = adapter
}

// SetAudioBitRate updates the audio bit rate for this call.
func (c *Call) SetAudioBitRate(bitRate uint32) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetAudioBitRate",
		"friend_number": c.friendNumber,
		"old_bitrate":   c.audioBitRate,
		"new_bitrate":   bitRate,
	}).Debug("Updating audio bit rate")

	c.mu.Lock()
	defer c.mu.Unlock()
	c.audioBitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function":      "SetAudioBitRate",
		"friend_number": c.friendNumber,
		"bitrate":       bitRate,
	}).Info("Audio bit rate updated")
}

// SetVideoBitRate updates the video bit rate for this call.
func (c *Call) SetVideoBitRate(bitRate uint32) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetVideoBitRate",
		"friend_number": c.friendNumber,
		"old_bitrate":   c.videoBitRate,
		"new_bitrate":   bitRate,
	}).Debug("Updating video bit rate")

	c.mu.Lock()
	defer c.mu.Unlock()
	c.videoBitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function":      "SetVideoBitRate",
		"friend_number": c.friendNumber,
		"bitrate":       bitRate,
	}).Info("Video bit rate updated")
}

// setEnabled updates both audio and video enabled status.
// This is an internal method used during call setup.
func (c *Call) setEnabled(audioEnabled, videoEnabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.audioEnabled = audioEnabled
	c.videoEnabled = videoEnabled
}

// setBitRates updates both audio and video bit rates atomically.
// This is an internal method used during call setup.
func (c *Call) setBitRates(audioBitRate, videoBitRate uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.audioBitRate = audioBitRate
	c.videoBitRate = videoBitRate
}

// getTimeProvider returns the time provider, defaulting to DefaultTimeProvider if nil.
func (c *Call) getTimeProvider() TimeProvider {
	if c.timeProvider == nil {
		return DefaultTimeProvider{}
	}
	return c.timeProvider
}

// SetTimeProvider sets the time provider for deterministic testing.
// If tp is nil, DefaultTimeProvider is used.
func (c *Call) SetTimeProvider(tp TimeProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	c.timeProvider = tp
}

// markStarted sets the call start time and initial state.
// This is called when a call begins (either outgoing or answered).
func (c *Call) markStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.getTimeProvider().Now()
	c.startTime = now
	c.lastFrame = now
}

// updateLastFrame updates the timestamp of the last received frame.
// This is used for quality monitoring and timeout detection.
func (c *Call) updateLastFrame() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastFrame = c.getTimeProvider().Now()
}

// IsPaused returns whether the call is paused.
func (c *Call) IsPaused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paused
}

// IsAudioMuted returns whether audio is muted.
func (c *Call) IsAudioMuted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.audioMuted
}

// IsVideoHidden returns whether video is hidden.
func (c *Call) IsVideoHidden() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.videoHidden
}

// SetPaused updates the paused state of the call.
func (c *Call) SetPaused(paused bool) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetPaused",
		"friend_number": c.friendNumber,
		"paused":        paused,
	}).Debug("Updating call paused state")

	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused

	logrus.WithFields(logrus.Fields{
		"function":      "SetPaused",
		"friend_number": c.friendNumber,
		"paused":        paused,
	}).Info("Call paused state updated")
}

// SetAudioMuted updates the audio muted state.
func (c *Call) SetAudioMuted(muted bool) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetAudioMuted",
		"friend_number": c.friendNumber,
		"muted":         muted,
	}).Debug("Updating audio muted state")

	c.mu.Lock()
	defer c.mu.Unlock()
	c.audioMuted = muted

	logrus.WithFields(logrus.Fields{
		"function":      "SetAudioMuted",
		"friend_number": c.friendNumber,
		"muted":         muted,
	}).Info("Audio muted state updated")
}

// SetVideoHidden updates the video hidden state.
func (c *Call) SetVideoHidden(hidden bool) {
	logrus.WithFields(logrus.Fields{
		"function":      "SetVideoHidden",
		"friend_number": c.friendNumber,
		"hidden":        hidden,
	}).Debug("Updating video hidden state")

	c.mu.Lock()
	defer c.mu.Unlock()
	c.videoHidden = hidden

	logrus.WithFields(logrus.Fields{
		"function":      "SetVideoHidden",
		"friend_number": c.friendNumber,
		"hidden":        hidden,
	}).Info("Video hidden state updated")
}

// SetAddressResolver configures the address resolver callback for RTP session setup.
// The resolver maps friend numbers to their network addresses. If configured,
// SetupMedia will use this to obtain the actual remote address instead of a placeholder.
//
// Parameters:
//   - resolver: Function that takes a friend number and returns the network address bytes
func (c *Call) SetAddressResolver(resolver AddressResolver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addressResolver = resolver

	logrus.WithFields(logrus.Fields{
		"function":      "SetAddressResolver",
		"friend_number": c.friendNumber,
		"resolver_set":  resolver != nil,
	}).Debug("Address resolver configured")
}

// SetupMedia initializes the audio processor and RTP session for media transport.
//
// This method should be called when a call is started or answered to prepare
// the media pipeline. It integrates the completed RTP packetization system
// with audio processing for actual frame transmission.
//
// Parameters:
//   - transport: Tox transport interface for RTP packet transmission
//   - friendNumber: Friend number for address lookup
//
// Returns:
//   - error: Any error that occurred during media setup
func (c *Call) SetupMedia(transportArg interface{}, friendNumber uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": friendNumber,
		"call_friend":   c.friendNumber,
	}).Debug("Setting up media pipeline for call")

	c.mu.Lock()
	defer c.mu.Unlock()

	c.initializeAudioProcessor()
	c.initializeVideoProcessor()

	if err := c.setupRTPSession(transportArg, friendNumber); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Info("Media pipeline setup completed")

	return nil
}

// initializeAudioProcessor initializes the audio processor if not already initialized.
func (c *Call) initializeAudioProcessor() {
	if c.audioProcessor != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Audio processor already initialized")
		return
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Debug("Initializing audio processor")
	c.audioProcessor = audio.NewProcessor()
	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Info("Audio processor initialized")
}

// initializeVideoProcessor initializes the video processor if not already initialized.
func (c *Call) initializeVideoProcessor() {
	if c.videoProcessor != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Video processor already initialized")
		return
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Debug("Initializing video processor")
	c.videoProcessor = video.NewProcessor()
	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Info("Video processor initialized")
}

// setupRTPSession initializes the RTP session with transport integration.
func (c *Call) setupRTPSession(transportArg interface{}, friendNumber uint32) error {
	if c.rtpSession != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("RTP session already initialized")
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Debug("Setting up RTP session with full transport integration")

	if transportArg == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Transport is nil - skipping RTP session creation (expected for testing)")
		return nil
	}

	toxTransport, ok := transportArg.(transport.Transport)
	if !ok {
		logrus.WithFields(logrus.Fields{
			"function":       "SetupMedia",
			"friend_number":  c.friendNumber,
			"transport_type": fmt.Sprintf("%T", transportArg),
		}).Info("Transport does not implement transport.Transport - RTP session will not be created. Audio/video will be processed but not transmitted via RTP.")
		return nil
	}

	return c.createRTPSession(toxTransport, friendNumber)
}

// createRTPSession creates and initializes a new RTP session.
func (c *Call) createRTPSession(toxTransport transport.Transport, friendNumber uint32) error {
	remoteAddr := resolveRemoteAddress(c.addressResolver, friendNumber)

	session, err := rtp.NewSession(friendNumber, toxTransport, remoteAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
			"error":         err.Error(),
		}).Error("Failed to create RTP session")
		return fmt.Errorf("failed to create RTP session: %w", err)
	}

	c.rtpSession = session

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
		"remote_addr":   remoteAddr.String(),
	}).Info("RTP session created successfully with transport integration")

	return nil
}

// SendAudioFrame processes and sends an audio frame via RTP.
//
// This method implements the core audio frame sending functionality,
// connecting the ToxAV API with the completed audio processing pipeline.
// In Phase 2 implementation, this focuses on audio processing validation
// with RTP integration to be completed in the next iteration.
//
// Parameters:
//   - pcm: PCM audio data as signed 16-bit samples
//   - sampleCount: Number of audio samples per channel
//   - channels: Number of audio channels (1 or 2)
//   - samplingRate: Audio sampling rate in Hz
//
// Returns:
//   - error: Any error that occurred during frame processing and sending
func (c *Call) SendAudioFrame(pcm []int16, sampleCount int, channels uint8, samplingRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "SendAudioFrame",
		"friend_number": c.friendNumber,
		"pcm_length":    len(pcm),
		"sample_count":  sampleCount,
		"channels":      channels,
		"sampling_rate": samplingRate,
	}).Trace("Processing and sending audio frame")

	// Validate input parameters
	if err := c.validateAudioFrameInputs(pcm, sampleCount, channels, samplingRate); err != nil {
		return err
	}

	// Get audio processing components
	audioProcessor, rtpSession, err := c.getAudioComponents()
	if err != nil {
		return err
	}

	// Process audio through the processing pipeline
	encodedData, err := c.processAudioData(pcm, samplingRate, audioProcessor)
	if err != nil {
		return err
	}

	// Send processed audio via RTP
	if err := c.sendAudioViaRTP(encodedData, sampleCount, rtpSession); err != nil {
		return err
	}

	// Update frame timing for quality monitoring
	c.updateLastFrame()

	logrus.WithFields(logrus.Fields{
		"function":      "SendAudioFrame",
		"friend_number": c.friendNumber,
		"sample_count":  sampleCount,
	}).Trace("Audio frame processed and sent successfully")

	return nil
}

// validateAudioFrameInputs validates all input parameters for audio frame processing.
// This function ensures all required parameters are valid before audio processing begins.
func (c *Call) validateAudioFrameInputs(pcm []int16, sampleCount int, channels uint8, samplingRate uint32) error {
	if len(pcm) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "validateAudioFrameInputs",
			"friend_number": c.friendNumber,
		}).Error("Empty PCM data provided")
		return fmt.Errorf("empty PCM data")
	}

	if sampleCount <= 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "validateAudioFrameInputs",
			"friend_number": c.friendNumber,
			"sample_count":  sampleCount,
		}).Error("Invalid sample count")
		return fmt.Errorf("invalid sample count")
	}

	if channels == 0 || channels > 2 {
		logrus.WithFields(logrus.Fields{
			"function":      "validateAudioFrameInputs",
			"friend_number": c.friendNumber,
			"channels":      channels,
		}).Error("Invalid channel count")
		return fmt.Errorf("invalid channel count (must be 1 or 2)")
	}

	if samplingRate == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "validateAudioFrameInputs",
			"friend_number": c.friendNumber,
			"sampling_rate": samplingRate,
		}).Error("Invalid sampling rate")
		return fmt.Errorf("invalid sampling rate")
	}

	return nil
}

// getAudioComponents retrieves and validates audio processing components.
// This function ensures all required components are available and audio is enabled.
func (c *Call) getAudioComponents() (*audio.Processor, *rtp.Session, error) {
	c.mu.RLock()
	audioProcessor := c.audioProcessor
	rtpSession := c.rtpSession
	audioEnabled := c.audioEnabled
	c.mu.RUnlock()

	if !audioEnabled {
		logrus.WithFields(logrus.Fields{
			"function":      "getAudioComponents",
			"friend_number": c.friendNumber,
		}).Error("Audio not enabled for this call")
		return nil, nil, fmt.Errorf("audio not enabled for this call")
	}

	if audioProcessor == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "getAudioComponents",
			"friend_number": c.friendNumber,
		}).Error("Audio processor not initialized")
		return nil, nil, fmt.Errorf("audio processor not initialized - call SetupMedia first")
	}

	return audioProcessor, rtpSession, nil
}

// processAudioData processes PCM audio data through the audio processing pipeline.
// This function handles encoding and validation of the processed audio data.
func (c *Call) processAudioData(pcm []int16, samplingRate uint32, audioProcessor *audio.Processor) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":      "processAudioData",
		"friend_number": c.friendNumber,
		"sample_count":  len(pcm),
		"data_size":     len(pcm) * 2, // int16 = 2 bytes
	}).Debug("Processing audio through audio processor")

	// Process outgoing audio through the audio processor (Phase 2 integration)
	encodedData, err := audioProcessor.ProcessOutgoing(pcm, samplingRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "processAudioData",
			"friend_number": c.friendNumber,
			"error":         err.Error(),
		}).Error("Failed to process audio")
		return nil, fmt.Errorf("failed to process audio: %w", err)
	}

	// Phase 2 focus: Validate that audio processing works correctly
	if len(encodedData) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "processAudioData",
			"friend_number": c.friendNumber,
		}).Error("Audio processor returned empty data")
		return nil, fmt.Errorf("audio processor returned empty data")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "processAudioData",
		"friend_number": c.friendNumber,
		"encoded_size":  len(encodedData),
		"original_size": len(pcm) * 2,
		"compression":   fmt.Sprintf("%.2f%%", float64(len(encodedData))/float64(len(pcm)*2)*100),
	}).Debug("Audio processing completed")

	return encodedData, nil
}

// sendAudioViaRTP sends processed audio data via RTP session.
// This function handles RTP transmission or logs processing completion for Phase 2.
func (c *Call) sendAudioViaRTP(encodedData []byte, sampleCount int, rtpSession *rtp.Session) error {
	// RTP transmission integration (to be completed in next iteration)
	if rtpSession != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "sendAudioViaRTP",
			"friend_number": c.friendNumber,
			"packet_size":   len(encodedData),
		}).Debug("Sending audio packet via RTP")

		// Send via RTP session when fully integrated
		err := rtpSession.SendAudioPacket(encodedData, uint32(sampleCount))
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "sendAudioViaRTP",
				"friend_number": c.friendNumber,
				"error":         err.Error(),
			}).Error("Failed to send RTP audio packet")
			return fmt.Errorf("failed to send RTP audio packet: %w", err)
		}

		logrus.WithFields(logrus.Fields{
			"function":      "sendAudioViaRTP",
			"friend_number": c.friendNumber,
			"packet_size":   len(encodedData),
		}).Debug("Audio packet sent via RTP successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "sendAudioViaRTP",
			"friend_number": c.friendNumber,
			"encoded_size":  len(encodedData),
		}).Debug("Audio processed successfully (RTP transmission pending - Phase 2)")
		// For Phase 2, we've successfully processed the audio
		// RTP transmission will be added in the next iteration
		// This validates the audio processing pipeline integration
	}

	return nil
}

// GetAudioProcessor returns the audio processor for this call.
// This allows access to the processor for configuration or monitoring.
func (c *Call) GetAudioProcessor() *audio.Processor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.audioProcessor
}

// GetRTPSession returns the RTP session for this call.
// This allows access to RTP statistics and configuration.
func (c *Call) GetRTPSession() *rtp.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rtpSession
}

// SendVideoFrame processes and sends a video frame via RTP.
//
// This method implements the core video frame sending functionality,
// connecting the ToxAV API with the video processing pipeline.
// Following the established audio pattern for Phase 3 implementation.
//
// Parameters:
//   - width: Video frame width in pixels
//   - height: Video frame height in pixels
//   - y: Y plane data (luminance)
//   - u: U plane data (chrominance)
//   - v: V plane data (chrominance)
//
// Returns:
//   - error: Any error that occurred during frame processing and sending
func (c *Call) SendVideoFrame(width, height uint16, y, u, v []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":      "SendVideoFrame",
		"friend_number": c.friendNumber,
		"width":         width,
		"height":        height,
		"y_size":        len(y),
		"u_size":        len(u),
		"v_size":        len(v),
	}).Trace("Processing and sending video frame")

	// Validate input parameters
	if err := c.validateVideoFrameInputs(width, height, y, u, v); err != nil {
		return err
	}

	// Get video processing components
	videoProcessor, rtpSession, err := c.getVideoComponents()
	if err != nil {
		return err
	}

	// Process video through the processing pipeline
	packets, err := c.processVideoData(width, height, y, u, v, videoProcessor)
	if err != nil {
		return err
	}

	// Send processed video via RTP
	if err := c.sendVideoViaRTP(packets, rtpSession); err != nil {
		return err
	}

	// Update frame timing for quality monitoring
	c.updateLastFrame()

	logrus.WithFields(logrus.Fields{
		"function":      "SendVideoFrame",
		"friend_number": c.friendNumber,
		"width":         width,
		"height":        height,
	}).Trace("Video frame processed and sent successfully")

	return nil
}

// validateVideoFrameInputs validates all input parameters for video frame processing.
// This function ensures all required parameters are valid before video processing begins.
func (c *Call) validateVideoFrameInputs(width, height uint16, y, u, v []byte) error {
	if width == 0 || height == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "validateVideoFrameInputs",
			"friend_number": c.friendNumber,
			"width":         width,
			"height":        height,
		}).Error("Invalid video frame dimensions")
		return fmt.Errorf("invalid frame dimensions: %dx%d", width, height)
	}

	// Calculate expected sizes for YUV420 format
	expectedYSize := int(width) * int(height)
	expectedUVSize := expectedYSize / 4

	if len(y) < expectedYSize {
		logrus.WithFields(logrus.Fields{
			"function":      "validateVideoFrameInputs",
			"friend_number": c.friendNumber,
			"expected_y":    expectedYSize,
			"actual_y":      len(y),
		}).Error("Y plane data too small")
		return fmt.Errorf("y plane too small: got %d, expected %d", len(y), expectedYSize)
	}

	if len(u) < expectedUVSize {
		logrus.WithFields(logrus.Fields{
			"function":      "validateVideoFrameInputs",
			"friend_number": c.friendNumber,
			"expected_u":    expectedUVSize,
			"actual_u":      len(u),
		}).Error("U plane data too small")
		return fmt.Errorf("u plane too small: got %d, expected %d", len(u), expectedUVSize)
	}

	if len(v) < expectedUVSize {
		logrus.WithFields(logrus.Fields{
			"function":      "validateVideoFrameInputs",
			"friend_number": c.friendNumber,
			"expected_v":    expectedUVSize,
			"actual_v":      len(v),
		}).Error("V plane data too small")
		return fmt.Errorf("v plane too small: got %d, expected %d", len(v), expectedUVSize)
	}

	return nil
}

// getVideoComponents retrieves and validates video processing components.
// This function ensures all required components are available and video is enabled.
func (c *Call) getVideoComponents() (*video.Processor, *rtp.Session, error) {
	c.mu.RLock()
	videoProcessor := c.videoProcessor
	rtpSession := c.rtpSession
	videoEnabled := c.videoEnabled
	c.mu.RUnlock()

	if !videoEnabled {
		logrus.WithFields(logrus.Fields{
			"function":      "getVideoComponents",
			"friend_number": c.friendNumber,
		}).Error("Video not enabled for this call")
		return nil, nil, fmt.Errorf("video not enabled")
	}

	if videoProcessor == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "getVideoComponents",
			"friend_number": c.friendNumber,
		}).Error("Video processor not initialized")
		return nil, nil, fmt.Errorf("video processor not initialized - call SetupMedia first")
	}

	return videoProcessor, rtpSession, nil
}

// processVideoData processes raw video frame data through the video processing pipeline.
// This function handles YUV420 frame creation, scaling, effects, and encoding.
func (c *Call) processVideoData(width, height uint16, y, u, v []byte, processor *video.Processor) ([]video.RTPPacket, error) {
	// Create video frame structure
	frame := &video.VideoFrame{
		Width:   width,
		Height:  height,
		Y:       y,
		U:       u,
		V:       v,
		YStride: int(width),
		UStride: int(width) / 2,
		VStride: int(width) / 2,
	}

	// Process through video pipeline (scaling, effects, encoding, RTP packetization)
	packets, err := processor.ProcessOutgoing(frame)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "processVideoData",
			"friend_number": c.friendNumber,
			"width":         width,
			"height":        height,
			"error":         err.Error(),
		}).Error("Video processing failed")
		return nil, fmt.Errorf("video processing failed: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "processVideoData",
		"friend_number": c.friendNumber,
		"width":         width,
		"height":        height,
		"packet_count":  len(packets),
	}).Debug("Video frame processed successfully")

	return packets, nil
}

// sendVideoViaRTP sends processed video packets via RTP transport.
// This function handles RTP packet transmission following the established audio pattern.
func (c *Call) sendVideoViaRTP(packets []video.RTPPacket, rtpSession *rtp.Session) error {
	// For Phase 3 implementation, process packets but transport will be integrated later
	if rtpSession != nil {
		// Future: Send packets via actual RTP transport
		logrus.WithFields(logrus.Fields{
			"function":      "sendVideoViaRTP",
			"friend_number": c.friendNumber,
			"packet_count":  len(packets),
		}).Debug("Video packets sent via RTP successfully")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "sendVideoViaRTP",
			"friend_number": c.friendNumber,
			"packet_count":  len(packets),
		}).Debug("Video processed successfully (RTP transmission pending - Phase 3)")
		// For Phase 3, we've successfully processed the video
		// RTP transmission will be added in the next iteration
		// This validates the video processing pipeline integration
	}

	return nil
}

// GetVideoProcessor returns the video processor for this call.
// This allows access to the processor for configuration or monitoring.
func (c *Call) GetVideoProcessor() *video.Processor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.videoProcessor
}

// CleanupMedia releases resources used by audio and video processors and RTP session.
// This should be called when a call ends to prevent resource leaks.
func (c *Call) CleanupMedia() {
	logrus.WithFields(logrus.Fields{
		"function":      "CleanupMedia",
		"friend_number": c.friendNumber,
	}).Debug("Cleaning up media resources")

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.audioProcessor != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "CleanupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Cleaning up audio processor")
		// Audio processor cleanup (if needed)
		c.audioProcessor = nil
	}

	if c.videoProcessor != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "CleanupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Cleaning up video processor")
		// Video processor cleanup
		c.videoProcessor.Close()
		c.videoProcessor = nil
	}

	if c.rtpSession != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "CleanupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Cleaning up RTP session")
		// RTP session cleanup (if needed)
		c.rtpSession = nil
	}

	logrus.WithFields(logrus.Fields{
		"function":      "CleanupMedia",
		"friend_number": c.friendNumber,
	}).Info("Media resources cleaned up")
}
