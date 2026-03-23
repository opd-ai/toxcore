// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PartitionDetectorConfig holds configuration for network partition detection.
type PartitionDetectorConfig struct {
	// CheckInterval is how often to check for partitions
	CheckInterval time.Duration
	// PartitionThreshold is the duration after which we consider ourselves partitioned
	// if no responses have been received
	PartitionThreshold time.Duration
	// MinHealthyNodes is the minimum number of responsive nodes to be considered healthy
	MinHealthyNodes int
	// RecoveryTimeout is how long to wait for re-bootstrap to succeed
	RecoveryTimeout time.Duration
}

// DefaultPartitionDetectorConfig returns sensible defaults for partition detection.
func DefaultPartitionDetectorConfig() *PartitionDetectorConfig {
	return &PartitionDetectorConfig{
		CheckInterval:      30 * time.Second,
		PartitionThreshold: 60 * time.Second,
		MinHealthyNodes:    1,
		RecoveryTimeout:    30 * time.Second,
	}
}

// PartitionState represents the current network partition status.
type PartitionState int

const (
	// StateHealthy means the network is functioning normally
	StateHealthy PartitionState = iota
	// StateWarning means we're seeing reduced connectivity
	StateWarning
	// StatePartitioned means we appear to be isolated from the network
	StatePartitioned
	// StateRecovering means we're attempting to recover from a partition
	StateRecovering
)

// String returns a human-readable string for the partition state.
func (ps PartitionState) String() string {
	switch ps {
	case StateHealthy:
		return "healthy"
	case StateWarning:
		return "warning"
	case StatePartitioned:
		return "partitioned"
	case StateRecovering:
		return "recovering"
	default:
		return "unknown"
	}
}

// PartitionDetector monitors the DHT routing table for signs of network partitions
// and triggers automatic recovery via re-bootstrap when partitions are detected.
//
//export ToxDHTPartitionDetector
type PartitionDetector struct {
	config          *PartitionDetectorConfig
	routingTable    *RoutingTable
	bootstrapper    *BootstrapManager
	gossipBootstrap *GossipBootstrap

	mu            sync.RWMutex
	state         PartitionState
	lastResponse  time.Time
	healthyCount  int
	recoveryCount int
	onPartition   func(PartitionState) // Callback for state changes
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool
	timeProvider  TimeProvider
}

// NewPartitionDetector creates a new partition detector.
//
//export ToxDHTPartitionDetectorNew
func NewPartitionDetector(
	routingTable *RoutingTable,
	bootstrapper *BootstrapManager,
	gossipBootstrap *GossipBootstrap,
	config *PartitionDetectorConfig,
) *PartitionDetector {
	if config == nil {
		config = DefaultPartitionDetectorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pd := &PartitionDetector{
		config:          config,
		routingTable:    routingTable,
		bootstrapper:    bootstrapper,
		gossipBootstrap: gossipBootstrap,
		state:           StateHealthy,
		lastResponse:    time.Now(),
		ctx:             ctx,
		cancel:          cancel,
		timeProvider:    nil,
	}

	return pd
}

// SetTimeProvider sets the time provider for deterministic testing.
func (pd *PartitionDetector) SetTimeProvider(tp TimeProvider) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.timeProvider = tp
}

// getTimeProvider returns the time provider, using default if nil.
func (pd *PartitionDetector) getTimeProvider() TimeProvider {
	if pd.timeProvider != nil {
		return pd.timeProvider
	}
	return getDefaultTimeProvider()
}

// Start begins the partition detection monitoring routine.
func (pd *PartitionDetector) Start() error {
	pd.mu.Lock()
	if pd.running {
		pd.mu.Unlock()
		return nil
	}
	pd.running = true
	pd.mu.Unlock()

	go pd.monitorRoutine()

	logrus.WithFields(logrus.Fields{
		"function":       "Start",
		"check_interval": pd.config.CheckInterval,
		"threshold":      pd.config.PartitionThreshold,
	}).Info("Partition detector started")

	return nil
}

// Stop halts the partition detection routine.
func (pd *PartitionDetector) Stop() {
	pd.mu.Lock()
	if !pd.running {
		pd.mu.Unlock()
		return
	}
	pd.running = false
	pd.cancel()
	pd.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "Stop",
	}).Info("Partition detector stopped")
}

// IsRunning returns whether the detector is currently monitoring.
func (pd *PartitionDetector) IsRunning() bool {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.running
}

// GetState returns the current partition state.
func (pd *PartitionDetector) GetState() PartitionState {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.state
}

// RecordResponse should be called when a response is received from any DHT node.
// This updates the last response time and healthy node count.
func (pd *PartitionDetector) RecordResponse() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.lastResponse = pd.getTimeProvider().Now()
}

// OnPartitionStateChange sets a callback for partition state changes.
func (pd *PartitionDetector) OnPartitionStateChange(callback func(PartitionState)) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.onPartition = callback
}

// GetHealthyNodeCount returns the number of healthy nodes in the routing table.
func (pd *PartitionDetector) GetHealthyNodeCount() int {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.healthyCount
}

// GetRecoveryCount returns the number of recovery attempts made.
func (pd *PartitionDetector) GetRecoveryCount() int {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.recoveryCount
}

// monitorRoutine periodically checks the routing table health.
func (pd *PartitionDetector) monitorRoutine() {
	ticker := time.NewTicker(pd.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pd.ctx.Done():
			return
		case <-ticker.C:
			pd.checkHealth()
		}
	}
}

// checkHealth evaluates the current routing table health and triggers recovery if needed.
func (pd *PartitionDetector) checkHealth() {
	healthyCount := pd.countHealthyNodes()

	pd.mu.Lock()
	pd.healthyCount = healthyCount
	now := pd.getTimeProvider().Now()
	timeSinceResponse := now.Sub(pd.lastResponse)
	oldState := pd.state
	pd.mu.Unlock()

	newState := pd.evaluateState(healthyCount, timeSinceResponse)

	if newState != oldState {
		pd.transitionState(oldState, newState)
	}

	// If partitioned and not recovering, attempt recovery
	if newState == StatePartitioned {
		go pd.attemptRecovery()
	}
}

// countHealthyNodes returns the number of nodes marked as "good" in the routing table.
func (pd *PartitionDetector) countHealthyNodes() int {
	if pd.routingTable == nil {
		return 0
	}

	count := 0
	for i := 0; i < 256; i++ {
		bucket := pd.routingTable.kBuckets[i]
		for _, node := range bucket.GetNodes() {
			if node.Status == StatusGood {
				count++
			}
		}
	}
	return count
}

// evaluateState determines the appropriate state based on health metrics.
func (pd *PartitionDetector) evaluateState(healthyCount int, timeSinceResponse time.Duration) PartitionState {
	pd.mu.RLock()
	currentState := pd.state
	pd.mu.RUnlock()

	// If recovering, stay in that state until recovery succeeds or fails
	if currentState == StateRecovering {
		if healthyCount >= pd.config.MinHealthyNodes {
			return StateHealthy
		}
		return StateRecovering
	}

	// No healthy nodes and no recent responses = partitioned
	if healthyCount == 0 && timeSinceResponse > pd.config.PartitionThreshold {
		return StatePartitioned
	}

	// Some nodes but below threshold = warning
	if healthyCount < pd.config.MinHealthyNodes {
		return StateWarning
	}

	return StateHealthy
}

// transitionState updates the state and triggers callbacks.
func (pd *PartitionDetector) transitionState(oldState, newState PartitionState) {
	pd.mu.Lock()
	pd.state = newState
	callback := pd.onPartition
	pd.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":  "transitionState",
		"old_state": oldState.String(),
		"new_state": newState.String(),
	}).Info("Partition state changed")

	if callback != nil {
		callback(newState)
	}
}

// attemptRecovery tries to recover from a network partition.
func (pd *PartitionDetector) attemptRecovery() {
	pd.mu.Lock()
	if pd.state != StatePartitioned {
		pd.mu.Unlock()
		return
	}
	pd.state = StateRecovering
	pd.recoveryCount++
	recoveryNum := pd.recoveryCount
	pd.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":       "attemptRecovery",
		"recovery_count": recoveryNum,
	}).Info("Attempting partition recovery")

	// Create a context with timeout for recovery
	ctx, cancel := context.WithTimeout(pd.ctx, pd.config.RecoveryTimeout)
	defer cancel()

	// Try gossip bootstrap first (uses peer exchange)
	if pd.gossipBootstrap != nil {
		if err := pd.tryGossipRecovery(ctx); err == nil {
			pd.handleRecoverySuccess("gossip")
			return
		}
	}

	// Fall back to regular bootstrap
	if pd.bootstrapper != nil {
		if err := pd.tryBootstrapRecovery(ctx); err == nil {
			pd.handleRecoverySuccess("bootstrap")
			return
		}
	}

	// Recovery failed
	pd.handleRecoveryFailure()
}

// tryGossipRecovery attempts to recover using gossip peer exchange.
func (pd *PartitionDetector) tryGossipRecovery(ctx context.Context) error {
	// Seed from any remaining routing table nodes
	pd.gossipBootstrap.SeedFromRoutingTable()

	// Try gossip-based bootstrap
	return pd.gossipBootstrap.BootstrapFromGossip(ctx)
}

// tryBootstrapRecovery attempts to recover using regular bootstrap nodes.
func (pd *PartitionDetector) tryBootstrapRecovery(ctx context.Context) error {
	// Get bootstrap nodes and try to connect
	nodes := pd.bootstrapper.GetNodes()
	if len(nodes) == 0 {
		return errors.New("no bootstrap nodes available")
	}

	// Trigger full bootstrap process
	return pd.bootstrapper.Bootstrap(ctx)
}

// handleRecoverySuccess processes successful recovery.
func (pd *PartitionDetector) handleRecoverySuccess(method string) {
	pd.mu.Lock()
	pd.state = StateHealthy
	pd.lastResponse = pd.getTimeProvider().Now()
	callback := pd.onPartition
	pd.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "handleRecoverySuccess",
		"method":   method,
	}).Info("Partition recovery successful")

	if callback != nil {
		callback(StateHealthy)
	}
}

// handleRecoveryFailure processes failed recovery attempt.
func (pd *PartitionDetector) handleRecoveryFailure() {
	pd.mu.Lock()
	pd.state = StatePartitioned // Back to partitioned state to retry later
	pd.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "handleRecoveryFailure",
	}).Warn("Partition recovery failed, will retry")
}

// ForceRecoveryAttempt manually triggers a recovery attempt.
// This can be called when an application detects connectivity issues.
func (pd *PartitionDetector) ForceRecoveryAttempt() {
	pd.mu.Lock()
	oldState := pd.state
	pd.state = StatePartitioned
	pd.mu.Unlock()

	if oldState != StatePartitioned && oldState != StateRecovering {
		logrus.WithFields(logrus.Fields{
			"function":  "ForceRecoveryAttempt",
			"old_state": oldState.String(),
		}).Info("Forced partition recovery triggered")
	}

	go pd.attemptRecovery()
}
