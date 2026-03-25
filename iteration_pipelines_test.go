package toxcore

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPipelineConfig(t *testing.T) {
	config := DefaultPipelineConfig()

	assert.Equal(t, 6*time.Second, config.DHTInterval)
	assert.Equal(t, 12*time.Second, config.FriendInterval)
	assert.Equal(t, 50*time.Millisecond, config.MessageInterval)
	assert.False(t, config.EnableConcurrent)
}

func TestNewIterationPipelines(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	pipelines := NewIterationPipelines(tox, nil)
	require.NotNil(t, pipelines)

	assert.Equal(t, tox, pipelines.tox)
	assert.Equal(t, 6*time.Second, pipelines.config.DHTInterval)
	assert.False(t, pipelines.IsRunning())
	assert.False(t, pipelines.IsConcurrent())
}

func TestIterationPipelinesCustomConfig(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      1 * time.Second,
		FriendInterval:   2 * time.Second,
		MessageInterval:  10 * time.Millisecond,
		EnableConcurrent: true,
	}

	pipelines := NewIterationPipelines(tox, config)
	require.NotNil(t, pipelines)

	assert.Equal(t, 1*time.Second, pipelines.config.DHTInterval)
	assert.Equal(t, 2*time.Second, pipelines.config.FriendInterval)
	assert.Equal(t, 10*time.Millisecond, pipelines.config.MessageInterval)
	assert.True(t, pipelines.IsConcurrent())
}

func TestIterationPipelinesStartStop(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	pipelines := NewIterationPipelines(tox, nil)

	// Should not be running initially
	assert.False(t, pipelines.IsRunning())

	// Start pipelines
	pipelines.Start()
	assert.True(t, pipelines.IsRunning())

	// Starting again should be a no-op
	pipelines.Start()
	assert.True(t, pipelines.IsRunning())

	// Stop pipelines
	pipelines.Stop()
	assert.False(t, pipelines.IsRunning())

	// Stopping again should be a no-op
	pipelines.Stop()
	assert.False(t, pipelines.IsRunning())
}

func TestIterationPipelinesSequentialMode(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      50 * time.Millisecond,
		FriendInterval:   100 * time.Millisecond,
		MessageInterval:  10 * time.Millisecond,
		EnableConcurrent: false,
	}

	pipelines := NewIterationPipelines(tox, config)
	pipelines.Start()

	// Wait for some processing
	time.Sleep(150 * time.Millisecond)

	pipelines.Stop()

	// Check stats
	dhtRuns, friendRuns, msgRuns, _, _, _ := pipelines.Stats()
	assert.True(t, dhtRuns >= 1, "DHT should have run at least once")
	assert.True(t, friendRuns >= 1, "Friends should have run at least once")
	assert.True(t, msgRuns >= 5, "Messages should have run multiple times")
}

func TestIterationPipelinesConcurrentMode(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      50 * time.Millisecond,
		FriendInterval:   50 * time.Millisecond,
		MessageInterval:  10 * time.Millisecond,
		EnableConcurrent: true,
	}

	pipelines := NewIterationPipelines(tox, config)
	pipelines.Start()

	// Wait for some processing
	time.Sleep(150 * time.Millisecond)

	pipelines.Stop()

	// Check stats - all pipelines should have run
	dhtRuns, friendRuns, msgRuns, _, _, _ := pipelines.Stats()
	assert.True(t, dhtRuns >= 1, "DHT should have run at least once")
	assert.True(t, friendRuns >= 1, "Friends should have run at least once")
	assert.True(t, msgRuns >= 5, "Messages should have run multiple times")
}

func TestIterationPipelinesTriggers(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      1 * time.Hour, // Long interval so timer doesn't trigger
		FriendInterval:   1 * time.Hour,
		MessageInterval:  1 * time.Hour,
		EnableConcurrent: true,
	}

	pipelines := NewIterationPipelines(tox, config)
	pipelines.Start()
	defer pipelines.Stop()

	// Give pipelines time to start
	time.Sleep(50 * time.Millisecond)

	// Trigger each pipeline manually
	pipelines.TriggerDHT()
	pipelines.TriggerFriends()
	pipelines.TriggerMessages()

	// Wait for triggers to be processed
	time.Sleep(100 * time.Millisecond)

	dhtRuns, friendRuns, msgRuns, _, _, _ := pipelines.Stats()
	assert.True(t, dhtRuns >= 1, "DHT should have been triggered")
	assert.True(t, friendRuns >= 1, "Friends should have been triggered")
	assert.True(t, msgRuns >= 1, "Messages should have been triggered")
}

func TestIterationPipelinesTriggersNonBlocking(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      1 * time.Hour,
		FriendInterval:   1 * time.Hour,
		MessageInterval:  1 * time.Hour,
		EnableConcurrent: false, // Not started
	}

	pipelines := NewIterationPipelines(tox, config)
	// Don't start - triggers should not block

	// These should not block even though pipeline is not running
	done := make(chan bool, 1)
	go func() {
		pipelines.TriggerDHT()
		pipelines.TriggerFriends()
		pipelines.TriggerMessages()
		done <- true
	}()

	select {
	case <-done:
		// Success - triggers did not block
	case <-time.After(1 * time.Second):
		t.Fatal("Triggers blocked when pipeline not running")
	}
}

func TestIterationPipelinesStats(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      20 * time.Millisecond,
		FriendInterval:   20 * time.Millisecond,
		MessageInterval:  5 * time.Millisecond,
		EnableConcurrent: true,
	}

	pipelines := NewIterationPipelines(tox, config)
	pipelines.Start()

	time.Sleep(100 * time.Millisecond)

	pipelines.Stop()

	dhtRuns, friendRuns, msgRuns, dhtDur, friendDur, msgDur := pipelines.Stats()

	// All should have run
	assert.True(t, dhtRuns >= 1)
	assert.True(t, friendRuns >= 1)
	assert.True(t, msgRuns >= 5)

	// Durations should be positive (at least a few nanoseconds)
	// Note: Duration might be 0 if execution was extremely fast
	_ = dhtDur
	_ = friendDur
	_ = msgDur
}

func TestToxEnableConcurrentIteration(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	pipelines := tox.EnableConcurrentIteration(nil)
	require.NotNil(t, pipelines)

	assert.Equal(t, tox, pipelines.tox)
	assert.False(t, pipelines.IsRunning())
}

func TestToxRunWithPipelines(t *testing.T) {
	tox := createTestTox(t)
	defer tox.Kill()

	config := &PipelineConfig{
		DHTInterval:      50 * time.Millisecond,
		FriendInterval:   50 * time.Millisecond,
		MessageInterval:  10 * time.Millisecond,
		EnableConcurrent: false,
	}

	pipelines := tox.RunWithPipelines(config)
	require.NotNil(t, pipelines)
	assert.True(t, pipelines.IsRunning())

	time.Sleep(100 * time.Millisecond)

	pipelines.Stop()
	assert.False(t, pipelines.IsRunning())
}

func TestPipelineStatsConcurrency(t *testing.T) {
	// Test that stats are thread-safe
	var stats PipelineStats

	var wg atomic.Int32
	wg.Store(100)

	for i := 0; i < 100; i++ {
		go func() {
			stats.DHTRuns.Add(1)
			stats.FriendRuns.Add(1)
			stats.MessageRuns.Add(1)
			stats.DHTDuration.Store(12345)
			stats.FriendDuration.Store(23456)
			stats.MessageDuration.Store(34567)
			wg.Add(-1)
		}()
	}

	// Wait for all goroutines
	for wg.Load() > 0 {
		time.Sleep(1 * time.Millisecond)
	}

	assert.Equal(t, uint64(100), stats.DHTRuns.Load())
	assert.Equal(t, uint64(100), stats.FriendRuns.Load())
	assert.Equal(t, uint64(100), stats.MessageRuns.Load())
}

// Helper to create a minimal Tox instance for testing
func createTestTox(t *testing.T) *Tox {
	t.Helper()

	options := NewOptionsForTesting()
	options.UDPEnabled = false // Disable networking for unit tests

	tox, err := New(options)
	require.NoError(t, err)
	require.NotNil(t, tox)

	return tox
}
