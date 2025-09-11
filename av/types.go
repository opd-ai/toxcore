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
	"sync"
	"time"

	"github.com/opd-ai/toxcore/av/audio"
	"github.com/opd-ai/toxcore/av/rtp"
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

	// Bit rate configuration
	audioBitRate uint32
	videoBitRate uint32

	// Timing information for quality monitoring
	startTime time.Time
	lastFrame time.Time

	// Audio processing and RTP transport for Phase 2 implementation
	audioProcessor *audio.Processor
	rtpSession     *rtp.Session

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
		audioBitRate: 0,
		videoBitRate: 0,
		startTime:    time.Time{},
		lastFrame:    time.Time{},
		// Audio components initialized when call starts
		audioProcessor: nil,
		rtpSession:     nil,
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

// markStarted sets the call start time and initial state.
// This is called when a call begins (either outgoing or answered).
func (c *Call) markStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startTime = time.Now()
	c.lastFrame = time.Now()
}

// updateLastFrame updates the timestamp of the last received frame.
// This is used for quality monitoring and timeout detection.
func (c *Call) updateLastFrame() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastFrame = time.Now()
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
func (c *Call) SetupMedia(transport interface{}, friendNumber uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": friendNumber,
		"call_friend":   c.friendNumber,
	}).Debug("Setting up media pipeline for call")

	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize audio processor (already implemented in Phase 2)
	if c.audioProcessor == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Initializing audio processor")
		c.audioProcessor = audio.NewProcessor()
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Info("Audio processor initialized")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Audio processor already initialized")
	}

	// Initialize RTP session (already implemented in Phase 2)
	if c.rtpSession == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("Setting up RTP session (simplified for Phase 2)")
		// For Phase 2, we'll use a simplified approach
		// The RTP session will be properly integrated with transport in next iteration
		// For now, we mark it as initialized to allow testing of the audio pipeline

		// Note: Full RTP transport integration will require:
		// 1. Proper transport.Transport interface implementation
		// 2. Friend address resolution for remoteAddr
		// 3. RTP packet handler registration

		// Placeholder to prevent nil pointer errors during testing
		// TODO: Complete RTP transport integration
		_ = transport
		_ = friendNumber

		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Warn("RTP session setup is placeholder (full integration pending)")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":      "SetupMedia",
			"friend_number": c.friendNumber,
		}).Debug("RTP session already initialized")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SetupMedia",
		"friend_number": c.friendNumber,
	}).Info("Media pipeline setup completed")

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

// CleanupMedia releases resources used by audio processor and RTP session.
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
