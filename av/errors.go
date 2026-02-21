package av

import "errors"

// Sentinel errors for av package operations.
// These errors enable reliable error classification using errors.Is().

// Call initiation errors.
var (
	// ErrFriendNotFound indicates the friend number is not known.
	ErrFriendNotFound = errors.New("friend not found")

	// ErrFriendNotConnected indicates the friend is not online.
	ErrFriendNotConnected = errors.New("friend not connected")

	// ErrCallAlreadyActive indicates a call already exists with this friend.
	ErrCallAlreadyActive = errors.New("call already active with this friend")

	// ErrInvalidBitRate indicates an invalid audio or video bit rate.
	ErrInvalidBitRate = errors.New("invalid bit rate")
)

// Answer errors.
var (
	// ErrNoIncomingCall indicates no pending call from this friend.
	ErrNoIncomingCall = errors.New("no incoming call from this friend")

	// ErrCodecInitialization indicates codec setup failed.
	ErrCodecInitialization = errors.New("codec initialization failed")
)

// Call control errors.
var (
	// ErrNoActiveCall indicates no call exists with this friend.
	ErrNoActiveCall = errors.New("no active call with this friend")

	// ErrCallNotPaused indicates the call is not currently paused.
	ErrCallNotPaused = errors.New("call is not paused")

	// ErrCallAlreadyPaused indicates the call is already paused.
	ErrCallAlreadyPaused = errors.New("call is already paused")

	// ErrAudioNotMuted indicates audio is not currently muted.
	ErrAudioNotMuted = errors.New("audio is not muted")

	// ErrAudioAlreadyMuted indicates audio is already muted.
	ErrAudioAlreadyMuted = errors.New("audio is already muted")

	// ErrVideoNotHidden indicates video is not currently hidden.
	ErrVideoNotHidden = errors.New("video is not hidden")

	// ErrVideoAlreadyHidden indicates video is already hidden.
	ErrVideoAlreadyHidden = errors.New("video is already hidden")

	// ErrInvalidTransition indicates an invalid state transition.
	ErrInvalidTransition = errors.New("invalid state transition")
)

// Send frame errors.
var (
	// ErrPayloadTypeDisabled indicates audio/video is disabled for this call.
	ErrPayloadTypeDisabled = errors.New("payload type disabled")

	// ErrRTPFailed indicates RTP transmission failed.
	ErrRTPFailed = errors.New("RTP transmission failed")
)

// Manager state errors.
var (
	// ErrManagerNotRunning indicates the manager has not been started.
	ErrManagerNotRunning = errors.New("manager is not running")

	// ErrManagerAlreadyRunning indicates the manager is already running.
	ErrManagerAlreadyRunning = errors.New("manager is already running")
)
