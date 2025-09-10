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
	"sync"
	"time"
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

// Call represents an individual audio/video call session.
//
// Each call maintains its own state, bit rates, timing information,
// and RTP session for media transport. The design ensures thread-safety
// through appropriate mutex usage following established patterns.
type Call struct {
	// Core call information
	friendNumber uint32
	state        CallState
	audioEnabled bool
	videoEnabled bool

	// Bit rate configuration
	audioBitRate uint32
	videoBitRate uint32

	// Timing information for quality monitoring
	startTime time.Time
	lastFrame time.Time

	// Thread safety
	mu sync.RWMutex
}

// NewCall creates a new call instance for the specified friend.
//
// The call starts in CallStateNone and must be started or answered
// to begin media transmission. This follows the established pattern
// of constructor functions in the toxcore-go codebase.
func NewCall(friendNumber uint32) *Call {
	return &Call{
		friendNumber: friendNumber,
		state:        CallStateNone,
		audioEnabled: false,
		videoEnabled: false,
		audioBitRate: 0,
		videoBitRate: 0,
		startTime:    time.Time{},
		lastFrame:    time.Time{},
	}
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
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

// SetAudioBitRate updates the audio bit rate for this call.
func (c *Call) SetAudioBitRate(bitRate uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.audioBitRate = bitRate
}

// SetVideoBitRate updates the video bit rate for this call.
func (c *Call) SetVideoBitRate(bitRate uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.videoBitRate = bitRate
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
