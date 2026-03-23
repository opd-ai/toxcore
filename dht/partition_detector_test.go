package dht

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionDetectorBasic(t *testing.T) {
	// Create test components
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := &PartitionDetectorConfig{
		CheckInterval:      100 * time.Millisecond,
		PartitionThreshold: 200 * time.Millisecond,
		MinHealthyNodes:    1,
		RecoveryTimeout:    100 * time.Millisecond,
	}

	pd := NewPartitionDetector(routingTable, nil, nil, config)
	require.NotNil(t, pd)

	assert.Equal(t, StateHealthy, pd.GetState())
	assert.Equal(t, 0, pd.GetHealthyNodeCount())
}

func TestPartitionDetectorStartStop(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := DefaultPartitionDetectorConfig()
	config.CheckInterval = 100 * time.Millisecond

	pd := NewPartitionDetector(routingTable, nil, nil, config)

	// Start
	err = pd.Start()
	require.NoError(t, err)
	assert.True(t, pd.IsRunning())

	// Starting again should be idempotent
	err = pd.Start()
	require.NoError(t, err)

	// Stop
	pd.Stop()
	assert.False(t, pd.IsRunning())

	// Stopping again should be idempotent
	pd.Stop()
	assert.False(t, pd.IsRunning())
}

func TestPartitionDetectorRecordResponse(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	pd := NewPartitionDetector(routingTable, nil, nil, nil)

	// Create a mock time provider
	mockTime := &mockTimeProvider{current: time.Now()}
	pd.SetTimeProvider(mockTime)

	// Record a response
	pd.RecordResponse()

	// Last response should be updated
	pd.mu.RLock()
	lastResponse := pd.lastResponse
	pd.mu.RUnlock()

	assert.Equal(t, mockTime.Now(), lastResponse)
}

func TestPartitionDetectorStateCallback(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := &PartitionDetectorConfig{
		CheckInterval:      50 * time.Millisecond,
		PartitionThreshold: 50 * time.Millisecond,
		MinHealthyNodes:    1,
		RecoveryTimeout:    50 * time.Millisecond,
	}

	pd := NewPartitionDetector(routingTable, nil, nil, config)

	// Track state changes
	var stateChanges []PartitionState
	var mu sync.Mutex
	pd.OnPartitionStateChange(func(state PartitionState) {
		mu.Lock()
		stateChanges = append(stateChanges, state)
		mu.Unlock()
	})

	// Set last response to a time in the past
	mockTime := &mockTimeProvider{current: time.Now()}
	pd.SetTimeProvider(mockTime)

	pd.mu.Lock()
	pd.lastResponse = mockTime.Now().Add(-100 * time.Millisecond) // Past threshold
	pd.mu.Unlock()

	// Start the detector
	err = pd.Start()
	require.NoError(t, err)
	defer pd.Stop()

	// Wait for the detector to check health
	time.Sleep(200 * time.Millisecond)

	// Should have detected partition and attempted recovery
	mu.Lock()
	changes := make([]PartitionState, len(stateChanges))
	copy(changes, stateChanges)
	mu.Unlock()

	// At minimum, should have transitioned to partitioned
	assert.True(t, len(changes) > 0, "Should have state changes")
}

func TestPartitionStateString(t *testing.T) {
	tests := []struct {
		state    PartitionState
		expected string
	}{
		{StateHealthy, "healthy"},
		{StateWarning, "warning"},
		{StatePartitioned, "partitioned"},
		{StateRecovering, "recovering"},
		{PartitionState(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.state.String())
	}
}

func TestPartitionDetectorCountHealthyNodes(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	// Add some nodes to the routing table
	for i := 0; i < 5; i++ {
		nodeKeyPair, _ := crypto.GenerateKeyPair()
		nodeID := crypto.NewToxID(nodeKeyPair.Public, [4]byte{})
		node := NewNode(*nodeID, nil)
		node.Status = StatusGood
		routingTable.AddNode(node)
	}

	pd := NewPartitionDetector(routingTable, nil, nil, nil)

	// Count should reflect the number of good nodes
	count := pd.countHealthyNodes()
	assert.Equal(t, 5, count)
}

func TestPartitionDetectorEvaluateState(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := &PartitionDetectorConfig{
		CheckInterval:      1 * time.Second,
		PartitionThreshold: 1 * time.Minute,
		MinHealthyNodes:    2,
		RecoveryTimeout:    30 * time.Second,
	}

	pd := NewPartitionDetector(routingTable, nil, nil, config)

	tests := []struct {
		name              string
		healthyCount      int
		timeSinceResponse time.Duration
		currentState      PartitionState
		expected          PartitionState
	}{
		{"healthy_above_threshold", 5, 30 * time.Second, StateHealthy, StateHealthy},
		{"warning_below_threshold", 1, 30 * time.Second, StateHealthy, StateWarning},
		{"partitioned_no_nodes_past_threshold", 0, 2 * time.Minute, StateHealthy, StatePartitioned},
		{"recovering_stays_recovering", 1, 30 * time.Second, StateRecovering, StateRecovering},
		{"recovering_to_healthy", 5, 30 * time.Second, StateRecovering, StateHealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd.mu.Lock()
			pd.state = tt.currentState
			pd.mu.Unlock()

			result := pd.evaluateState(tt.healthyCount, tt.timeSinceResponse)
			assert.Equal(t, tt.expected, result, "Test: %s", tt.name)
		})
	}
}

func TestPartitionDetectorForceRecovery(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := &PartitionDetectorConfig{
		CheckInterval:      1 * time.Second,
		PartitionThreshold: 1 * time.Second,
		MinHealthyNodes:    1,
		RecoveryTimeout:    100 * time.Millisecond,
	}

	pd := NewPartitionDetector(routingTable, nil, nil, config)

	// Force recovery
	pd.ForceRecoveryAttempt()

	// Wait for recovery attempt to complete
	time.Sleep(200 * time.Millisecond)

	// Should have incremented recovery count
	assert.GreaterOrEqual(t, pd.GetRecoveryCount(), 1)
}

func TestPartitionRecovery(t *testing.T) {
	// This test validates the full partition recovery flow
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	toxID := crypto.NewToxID(keyPair.Public, [4]byte{})
	routingTable := NewRoutingTable(*toxID, 8)

	config := &PartitionDetectorConfig{
		CheckInterval:      50 * time.Millisecond,
		PartitionThreshold: 100 * time.Millisecond,
		MinHealthyNodes:    1,
		RecoveryTimeout:    200 * time.Millisecond,
	}

	// Create gossip bootstrap for recovery
	gossipBootstrap := NewGossipBootstrap(*toxID, nil, routingTable, nil)

	pd := NewPartitionDetector(routingTable, nil, gossipBootstrap, config)

	// Set up mock time provider
	mockTime := &mockTimeProvider{current: time.Now()}
	pd.SetTimeProvider(mockTime)

	// Force into partitioned state
	pd.mu.Lock()
	pd.state = StatePartitioned
	pd.lastResponse = mockTime.Now().Add(-200 * time.Millisecond)
	pd.mu.Unlock()

	// Attempt recovery (will fail due to no peers, but exercises the code path)
	pd.attemptRecovery()

	// After failed recovery, should be back in partitioned state
	assert.Equal(t, StatePartitioned, pd.GetState())
	assert.GreaterOrEqual(t, pd.GetRecoveryCount(), 1)
}

func TestDefaultPartitionDetectorConfig(t *testing.T) {
	config := DefaultPartitionDetectorConfig()

	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.Equal(t, 60*time.Second, config.PartitionThreshold)
	assert.Equal(t, 1, config.MinHealthyNodes)
	assert.Equal(t, 30*time.Second, config.RecoveryTimeout)
}
