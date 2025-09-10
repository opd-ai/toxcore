package av

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Manager handles multiple concurrent audio/video calls and integrates with Tox.
//
// The Manager follows established patterns from the toxcore-go codebase:
// - Reuses existing transport and crypto infrastructure
// - Thread-safe operations with appropriate mutex usage
// - Interface-based design for testability
// - Integration with existing friend management system
type Manager struct {
	// Core integration - transport for signaling and media
	transport TransportInterface

	// Friend lookup function for packet routing
	// This function should return the friend's network address given their friend number
	friendAddressLookup func(friendNumber uint32) ([]byte, error)

	// Active calls mapping friend numbers to call instances
	calls map[uint32]*Call

	// State management
	running bool

	// Thread safety following established patterns
	mu sync.RWMutex

	// Iteration timing for integration with Tox event loop
	iterationInterval time.Duration

	// Call ID generation for unique call identification
	nextCallID uint32
}

// TransportInterface defines the minimal interface needed for AV signaling.
// This allows the manager to work with any transport implementation without
// tight coupling to specific transport types.
type TransportInterface interface {
	// Send sends a packet to the specified address
	Send(packetType byte, data []byte, addr []byte) error
	
	// RegisterHandler registers a handler for specific packet types
	RegisterHandler(packetType byte, handler func(data []byte, addr []byte) error)
}

// NewManager creates a new ToxAV manager instance with transport integration.
//
// The manager integrates with an existing transport and friend lookup system
// to provide audio/video calling capabilities. This follows the established
// pattern of constructor functions in toxcore-go.
//
// Parameters:
//   - transport: The transport interface for signaling and media
//   - friendAddressLookup: Function to get friend network addresses
//
// Returns:
//   - *Manager: The new manager instance
//   - error: Any error that occurred during setup
func NewManager(transport TransportInterface, friendAddressLookup func(uint32) ([]byte, error)) (*Manager, error) {
	if transport == nil {
		return nil, errors.New("transport interface cannot be nil")
	}
	if friendAddressLookup == nil {
		return nil, errors.New("friend address lookup function cannot be nil")
	}

	manager := &Manager{
		transport:           transport,
		friendAddressLookup: friendAddressLookup,
		calls:               make(map[uint32]*Call),
		running:             false,
		iterationInterval:   20 * time.Millisecond, // 50 FPS, typical for A/V applications
		nextCallID:          1,
	}

	// Register packet handlers for AV signaling
	manager.registerPacketHandlers()

	return manager, nil
}

// Start begins the manager's operation.
//
// This method should be called after creating the manager and before
// starting any calls. It follows the established pattern of lifecycle
// management in toxcore-go components.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return errors.New("manager is already running")
	}

	m.running = true
	return nil
}

// Stop gracefully shuts down the manager.
//
// This method ends all active calls and stops the manager operation.
// It follows the established cleanup patterns in toxcore-go.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	// End all active calls
	for friendNumber, call := range m.calls {
		call.SetState(CallStateFinished)
		delete(m.calls, friendNumber)
	}

	m.running = false
	return nil
}

// IsRunning returns whether the manager is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IterationInterval returns the recommended interval for calling Iterate().
//
// This follows the established pattern in toxcore-go where components
// provide their own iteration timing requirements.
func (m *Manager) IterationInterval() time.Duration {
	return m.iterationInterval
}

// Iterate performs one iteration of the manager's event loop.
//
// This method should be called regularly (at IterationInterval) to
// process A/V events, handle timeouts, and maintain call state.
// It follows the established iteration pattern in toxcore-go.
func (m *Manager) Iterate() {
	m.mu.RLock()
	calls := make([]*Call, 0, len(m.calls))
	for _, call := range m.calls {
		calls = append(calls, call)
	}
	running := m.running
	m.mu.RUnlock()

	if !running {
		return
	}

	// Process each active call
	for _, call := range calls {
		m.processCall(call)
	}
}

// processCall handles the processing for an individual call.
//
// This method checks for timeouts, processes incoming media,
// and handles state transitions. It's called during iteration
// for each active call.
func (m *Manager) processCall(call *Call) {
	// TODO: Process incoming audio/video frames
	// TODO: Handle call timeouts
	// TODO: Process quality monitoring

	// For now, just ensure the call state is valid
	state := call.GetState()
	if state == CallStateError {
		// Remove failed calls
		m.mu.Lock()
		delete(m.calls, call.GetFriendNumber())
		m.mu.Unlock()
	}
}

// StartCall initiates an outgoing audio/video call to a friend.
//
// This method creates a new call instance and begins the call setup
// process. It follows error handling patterns established in toxcore-go.
//
// Parameters:
//   - friendNumber: The friend to call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call setup
func (m *Manager) StartCall(friendNumber uint32, audioBitRate, videoBitRate uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return errors.New("manager is not running")
	}

	// Check if call already exists
	if _, exists := m.calls[friendNumber]; exists {
		return fmt.Errorf("call already exists for friend %d", friendNumber)
	}

	// Validate bit rates
	if audioBitRate == 0 && videoBitRate == 0 {
		return errors.New("at least one of audio or video must be enabled")
	}

	// Create new call
	call := NewCall(friendNumber)
	call.setEnabled(audioBitRate > 0, videoBitRate > 0)
	call.setBitRates(audioBitRate, videoBitRate)
	call.markStarted()

	// TODO: Send call request through Tox transport
	// TODO: Set up RTP session for media transport

	// For now, just mark as sending based on enabled media
	var state CallState = CallStateNone
	if audioBitRate > 0 && videoBitRate > 0 {
		state = CallStateSendingAudio | CallStateSendingVideo
	} else if audioBitRate > 0 {
		state = CallStateSendingAudio
	} else if videoBitRate > 0 {
		state = CallStateSendingVideo
	}

	call.SetState(state)
	m.calls[friendNumber] = call

	return nil
}

// AnswerCall accepts an incoming audio/video call.
//
// This method responds to an incoming call request and sets up
// the media streams with the specified bit rates.
//
// Parameters:
//   - friendNumber: The friend who initiated the call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call answer
func (m *Manager) AnswerCall(friendNumber uint32, audioBitRate, videoBitRate uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return errors.New("manager is not running")
	}

	// Check if call exists and is in the right state
	call, exists := m.calls[friendNumber]
	if !exists {
		return fmt.Errorf("no incoming call from friend %d", friendNumber)
	}

	if call.GetState() != CallStateNone {
		return fmt.Errorf("call with friend %d is not in answerable state", friendNumber)
	}

	// Validate bit rates
	if audioBitRate == 0 && videoBitRate == 0 {
		return errors.New("at least one of audio or video must be enabled")
	}

	// Update call configuration
	call.setEnabled(audioBitRate > 0, videoBitRate > 0)
	call.setBitRates(audioBitRate, videoBitRate)
	call.markStarted()

	// TODO: Send call answer through Tox transport
	// TODO: Set up RTP session for media transport

	// Set state based on enabled media
	var state CallState = CallStateNone
	if audioBitRate > 0 && videoBitRate > 0 {
		state = CallStateAcceptingAudio | CallStateAcceptingVideo
	} else if audioBitRate > 0 {
		state = CallStateAcceptingAudio
	} else if videoBitRate > 0 {
		state = CallStateAcceptingVideo
	}

	call.SetState(state)
	return nil
}

// EndCall terminates an active call.
//
// This method gracefully ends a call and cleans up associated resources.
//
// Parameters:
//   - friendNumber: The friend to end the call with
//
// Returns:
//   - error: Any error that occurred during call termination
func (m *Manager) EndCall(friendNumber uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[friendNumber]
	if !exists {
		return fmt.Errorf("no active call with friend %d", friendNumber)
	}

	// TODO: Send call end signal through Tox transport
	// TODO: Clean up RTP session

	call.SetState(CallStateFinished)
	delete(m.calls, friendNumber)

	return nil
}

// GetCall returns the call instance for a friend, if it exists.
//
// This method provides read-only access to call information.
//
// Parameters:
//   - friendNumber: The friend to get call information for
//
// Returns:
//   - *Call: The call instance, or nil if no call exists
//   - bool: Whether a call exists
func (m *Manager) GetCall(friendNumber uint32) (*Call, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	call, exists := m.calls[friendNumber]
	return call, exists
}

// GetActiveCalls returns a list of all active call friend numbers.
//
// This method provides a snapshot of current call state for monitoring
// and management purposes.
//
// Returns:
//   - []uint32: List of friend numbers with active calls
func (m *Manager) GetActiveCalls() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	friendNumbers := make([]uint32, 0, len(m.calls))
	for friendNumber := range m.calls {
		friendNumbers = append(friendNumbers, friendNumber)
	}

	return friendNumbers
}

// SetAudioBitRate updates the audio bit rate for an active call.
//
// This method allows dynamic adjustment of audio quality during a call.
//
// Parameters:
//   - friendNumber: The friend to update audio bit rate for
//   - bitRate: New audio bit rate in bits/second (0 to disable audio)
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (m *Manager) SetAudioBitRate(friendNumber uint32, bitRate uint32) error {
	m.mu.RLock()
	call, exists := m.calls[friendNumber]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no active call with friend %d", friendNumber)
	}

	// TODO: Update audio encoder with new bit rate
	// TODO: Send bit rate update through RTP

	call.SetAudioBitRate(bitRate)
	return nil
}

// SetVideoBitRate updates the video bit rate for an active call.
//
// This method allows dynamic adjustment of video quality during a call.
//
// Parameters:
//   - friendNumber: The friend to update video bit rate for
//   - bitRate: New video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (m *Manager) SetVideoBitRate(friendNumber uint32, bitRate uint32) error {
	m.mu.RLock()
	call, exists := m.calls[friendNumber]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no active call with friend %d", friendNumber)
	}

	// TODO: Update video encoder with new bit rate
	// TODO: Send bit rate update through RTP

	call.SetVideoBitRate(bitRate)
	return nil
}
