package toxcore

import (
	"errors"
	"fmt"
	"sync"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
)

// ToxAV represents an audio/video instance that integrates with a Tox instance.
//
// ToxAV provides the high-level API for audio/video calling functionality.
// It follows the established patterns in toxcore-go:
// - Callback-based event handling
// - Thread-safe operations
// - Integration with existing Tox infrastructure
// - Compatibility with libtoxcore ToxAV API
type ToxAV struct {
	// Core integration
	tox  *Tox
	impl *avpkg.Manager

	// Thread safety
	mu sync.RWMutex

	// Callbacks matching libtoxcore ToxAV API exactly
	callCb         func(friendNumber uint32, audioEnabled, videoEnabled bool)
	callStateCb    func(friendNumber uint32, state avpkg.CallState)
	audioBitRateCb func(friendNumber uint32, bitRate uint32)
	videoBitRateCb func(friendNumber uint32, bitRate uint32)
	audioReceiveCb func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)
	videoReceiveCb func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)
}

// NewToxAV creates a new ToxAV instance from an existing Tox instance.
//
// The ToxAV instance will integrate with the Tox instance's networking,
// crypto, and friend management systems. This follows the established
// pattern of constructor functions in toxcore-go.
//
// Parameters:
//   - tox: The Tox instance to integrate with
//
// Returns:
//   - *ToxAV: The new ToxAV instance
//   - error: Any error that occurred during setup
func NewToxAV(tox *Tox) (*ToxAV, error) {
	if tox == nil {
		return nil, errors.New("tox instance cannot be nil")
	}

	// Create the underlying manager
	manager, err := avpkg.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create AV manager: %w", err)
	}

	// Start the manager
	if err := manager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start AV manager: %w", err)
	}

	toxav := &ToxAV{
		tox:  tox,
		impl: manager,
	}

	return toxav, nil
}

// Kill gracefully shuts down the ToxAV instance.
//
// This method ends all active calls and releases resources.
// It follows the established cleanup patterns in toxcore-go.
func (av *ToxAV) Kill() {
	av.mu.Lock()
	defer av.mu.Unlock()

	if av.impl != nil {
		av.impl.Stop()
		av.impl = nil
	}
}

// Iterate performs one iteration of the ToxAV event loop.
//
// This method should be called regularly (at IterationInterval) to
// process audio/video events and maintain call state. It follows
// the established iteration pattern in toxcore-go.
func (av *ToxAV) Iterate() {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl != nil {
		impl.Iterate()
	}
}

// IterationInterval returns the recommended interval for calling Iterate().
//
// This follows the established pattern in toxcore-go where components
// provide their own iteration timing requirements.
func (av *ToxAV) IterationInterval() time.Duration {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl != nil {
		return impl.IterationInterval()
	}
	return 20 * time.Millisecond
}

// Call initiates an audio/video call to a friend.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call initiation
func (av *ToxAV) Call(friendNumber uint32, audioBitRate, videoBitRate uint32) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	return impl.StartCall(friendNumber, audioBitRate, videoBitRate)
}

// Answer accepts an incoming audio/video call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend who initiated the call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call answer
func (av *ToxAV) Answer(friendNumber uint32, audioBitRate, videoBitRate uint32) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	return impl.AnswerCall(friendNumber, audioBitRate, videoBitRate)
}

// CallControl sends a call control command.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the control command to
//   - control: The control command to send
//
// Returns:
//   - error: Any error that occurred during control command sending
func (av *ToxAV) CallControl(friendNumber uint32, control avpkg.CallControl) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	switch control {
	case avpkg.CallControlCancel:
		return impl.EndCall(friendNumber)
	case avpkg.CallControlResume, avpkg.CallControlPause:
		// TODO: Implement pause/resume functionality
		return errors.New("pause/resume not yet implemented")
	case avpkg.CallControlMuteAudio, avpkg.CallControlUnmuteAudio:
		// TODO: Implement audio mute/unmute
		return errors.New("audio mute/unmute not yet implemented")
	case avpkg.CallControlHideVideo, avpkg.CallControlShowVideo:
		// TODO: Implement video hide/show
		return errors.New("video hide/show not yet implemented")
	default:
		return fmt.Errorf("unknown call control: %d", control)
	}
}

// AudioSetBitRate sets the audio bit rate for an active call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to update audio bit rate for
//   - bitRate: New audio bit rate in bits/second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (av *ToxAV) AudioSetBitRate(friendNumber uint32, bitRate uint32) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	return impl.SetAudioBitRate(friendNumber, bitRate)
}

// VideoSetBitRate sets the video bit rate for an active call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to update video bit rate for
//   - bitRate: New video bit rate in bits/second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (av *ToxAV) VideoSetBitRate(friendNumber uint32, bitRate uint32) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	return impl.SetVideoBitRate(friendNumber, bitRate)
}

// AudioSendFrame sends an audio frame to a friend.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the audio frame to
//   - pcm: PCM audio data as signed 16-bit samples
//   - sampleCount: Number of audio samples per channel
//   - channels: Number of audio channels (1 or 2)
//   - samplingRate: Audio sampling rate in Hz
//
// Returns:
//   - error: Any error that occurred during frame sending
func (av *ToxAV) AudioSendFrame(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	// TODO: Implement audio frame encoding and sending
	// This will be implemented in Phase 2: Audio Implementation
	return errors.New("audio frame sending not yet implemented")
}

// VideoSendFrame sends a video frame to a friend.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the video frame to
//   - width: Video frame width in pixels
//   - height: Video frame height in pixels
//   - y: Y plane data (luminance)
//   - u: U plane data (chrominance)
//   - v: V plane data (chrominance)
//
// Returns:
//   - error: Any error that occurred during frame sending
func (av *ToxAV) VideoSendFrame(friendNumber uint32, width, height uint16, y, u, v []byte) error {
	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		return errors.New("ToxAV instance has been destroyed")
	}

	// TODO: Implement video frame encoding and sending
	// This will be implemented in Phase 3: Video Implementation
	return errors.New("video frame sending not yet implemented")
}

// CallbackCall sets the callback for incoming call requests.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when a call request is received
func (av *ToxAV) CallbackCall(callback func(friendNumber uint32, audioEnabled, videoEnabled bool)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.callCb = callback
}

// CallbackCallState sets the callback for call state changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when call state changes
func (av *ToxAV) CallbackCallState(callback func(friendNumber uint32, state avpkg.CallState)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.callStateCb = callback
}

// CallbackAudioBitRate sets the callback for audio bit rate changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when audio bit rate changes
func (av *ToxAV) CallbackAudioBitRate(callback func(friendNumber uint32, bitRate uint32)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.audioBitRateCb = callback
}

// CallbackVideoBitRate sets the callback for video bit rate changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when video bit rate changes
func (av *ToxAV) CallbackVideoBitRate(callback func(friendNumber uint32, bitRate uint32)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.videoBitRateCb = callback
}

// CallbackAudioReceiveFrame sets the callback for incoming audio frames.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when an audio frame is received
func (av *ToxAV) CallbackAudioReceiveFrame(callback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.audioReceiveCb = callback
}

// CallbackVideoReceiveFrame sets the callback for incoming video frames.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when a video frame is received
func (av *ToxAV) CallbackVideoReceiveFrame(callback func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.videoReceiveCb = callback
}
