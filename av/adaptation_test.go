package av

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkQualityString(t *testing.T) {
	tests := []struct {
		name     string
		quality  NetworkQuality
		expected string
	}{
		{"excellent quality", NetworkExcellent, "excellent"},
		{"good quality", NetworkGood, "good"},
		{"fair quality", NetworkFair, "fair"},
		{"poor quality", NetworkPoor, "poor"},
		{"unknown quality", NetworkQuality(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.quality.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultAdaptationConfig(t *testing.T) {
	config := DefaultAdaptationConfig()
	require.NotNil(t, config)

	// Verify sensible defaults
	assert.Equal(t, 2*time.Second, config.StatsInterval)
	assert.Equal(t, 10*time.Second, config.AdaptationWindow)

	// Verify quality thresholds are in ascending order
	assert.Less(t, config.GoodLossThreshold, config.FairLossThreshold)
	assert.Less(t, config.FairLossThreshold, config.PoorLossThreshold)
	assert.Less(t, config.GoodJitterThreshold, config.FairJitterThreshold)
	assert.Less(t, config.FairJitterThreshold, config.PoorJitterThreshold)

	// Verify bitrate limits are reasonable
	assert.Greater(t, config.MinAudioBitRate, uint32(0))
	assert.Greater(t, config.MaxAudioBitRate, config.MinAudioBitRate)
	assert.Greater(t, config.MinVideoBitRate, uint32(0))
	assert.Greater(t, config.MaxVideoBitRate, config.MinVideoBitRate)

	// Verify AIMD parameters
	assert.Greater(t, config.IncreaseStep, 0.0)
	assert.Less(t, config.IncreaseStep, 1.0)
	assert.Greater(t, config.DecreaseMultiplier, 0.0)
	assert.Less(t, config.DecreaseMultiplier, 1.0)
}

func TestNewBitrateAdapter(t *testing.T) {
	tests := []struct {
		name          string
		config        *AdaptationConfig
		initialAudio  uint32
		initialVideo  uint32
		expectDefault bool
	}{
		{
			name:          "with custom config",
			config:        &AdaptationConfig{StatsInterval: 1 * time.Second},
			initialAudio:  32000,
			initialVideo:  500000,
			expectDefault: false,
		},
		{
			name:          "with nil config uses default",
			config:        nil,
			initialAudio:  16000,
			initialVideo:  100000,
			expectDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewBitrateAdapter(tt.config, tt.initialAudio, tt.initialVideo)
			require.NotNil(t, adapter)

			// Verify initial state
			assert.Equal(t, NetworkGood, adapter.currentQuality)
			assert.Equal(t, tt.initialAudio, adapter.audioBitRate)
			assert.Equal(t, tt.initialVideo, adapter.videoBitRate)
			assert.Equal(t, uint64(0), adapter.adaptationCount)

			// Verify config handling
			if tt.expectDefault {
				// Should use default config
				assert.Equal(t, 2*time.Second, adapter.config.StatsInterval)
			} else {
				// Should use provided config
				assert.Equal(t, tt.config.StatsInterval, adapter.config.StatsInterval)
			}
		})
	}
}

func TestSetCallbacks(t *testing.T) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	// Initially no callbacks
	assert.Nil(t, adapter.audioBitRateCb)
	assert.Nil(t, adapter.videoBitRateCb)
	assert.Nil(t, adapter.qualityCb)

	// Set callbacks
	audioCallback := func(bitRate uint32) { /* callback set */ }
	videoCallback := func(bitRate uint32) { /* callback set */ }
	qualityCallback := func(quality NetworkQuality) { /* callback set */ }

	adapter.SetCallbacks(audioCallback, videoCallback, qualityCallback)

	// Verify callbacks are set
	assert.NotNil(t, adapter.audioBitRateCb)
	assert.NotNil(t, adapter.videoBitRateCb)
	assert.NotNil(t, adapter.qualityCb)
}

func TestAssessNetworkQuality(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	tests := []struct {
		name        string
		lossPercent float64
		jitter      time.Duration
		expected    NetworkQuality
	}{
		{
			name:        "excellent conditions",
			lossPercent: 0.5,
			jitter:      30 * time.Millisecond,
			expected:    NetworkExcellent,
		},
		{
			name:        "good conditions",
			lossPercent: 1.5,
			jitter:      70 * time.Millisecond,
			expected:    NetworkGood,
		},
		{
			name:        "fair conditions",
			lossPercent: 4.0,
			jitter:      120 * time.Millisecond,
			expected:    NetworkFair,
		},
		{
			name:        "poor conditions",
			lossPercent: 6.0,
			jitter:      200 * time.Millisecond,
			expected:    NetworkPoor,
		},
		{
			name:        "mixed conditions - poor jitter wins",
			lossPercent: 0.5,                    // Excellent
			jitter:      200 * time.Millisecond, // Poor
			expected:    NetworkPoor,
		},
		{
			name:        "mixed conditions - poor loss wins",
			lossPercent: 6.0,                   // Poor
			jitter:      30 * time.Millisecond, // Excellent
			expected:    NetworkPoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quality := adapter.assessNetworkQuality(tt.lossPercent, tt.jitter)
			assert.Equal(t, tt.expected, quality,
				"Expected %s for loss=%.1f%% jitter=%v, got %s",
				tt.expected.String(), tt.lossPercent, tt.jitter, quality.String())
		})
	}
}

func TestUpdateNetworkStatsQualityChange(t *testing.T) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	// Set up quality callback to capture changes
	var capturedQuality NetworkQuality
	qualityChanged := false
	adapter.SetCallbacks(nil, nil, func(quality NetworkQuality) {
		capturedQuality = quality
		qualityChanged = true
	})

	timestamp := time.Now()

	// Update with poor network conditions
	adapted, err := adapter.UpdateNetworkStats(100, 90, 10, 200*time.Millisecond, timestamp)
	require.NoError(t, err)

	// Should detect quality change but not adapt yet (within adaptation window)
	assert.False(t, adapted)
	assert.Equal(t, NetworkPoor, adapter.GetNetworkQuality())

	// Give callback time to execute
	time.Sleep(10 * time.Millisecond)
	assert.True(t, qualityChanged)
	assert.Equal(t, NetworkPoor, capturedQuality)
}

func TestUpdateNetworkStatsAdaptation(t *testing.T) {
	config := DefaultAdaptationConfig()
	config.AdaptationWindow = 100 * time.Millisecond // Short window for testing
	adapter := NewBitrateAdapter(config, 32000, 500000)

	// Set up callbacks to capture changes
	var capturedAudioBitRate, capturedVideoBitRate uint32
	audioCbCalled := false
	videoCbCalled := false

	adapter.SetCallbacks(
		func(bitRate uint32) {
			capturedAudioBitRate = bitRate
			audioCbCalled = true
		},
		func(bitRate uint32) {
			capturedVideoBitRate = bitRate
			videoCbCalled = true
		},
		nil,
	)

	timestamp := time.Now()

	// First update - should not adapt (within window)
	adapted, err := adapter.UpdateNetworkStats(100, 90, 10, 200*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.False(t, adapted)

	// Wait for adaptation window to pass
	time.Sleep(150 * time.Millisecond)
	timestamp = time.Now()

	// Second update - should adapt due to poor quality
	adapted, err = adapter.UpdateNetworkStats(100, 90, 10, 200*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.True(t, adapted)

	// Verify bitrates decreased
	currentAudio, currentVideo := adapter.GetCurrentBitrates()
	assert.Less(t, currentAudio, uint32(32000))
	assert.Less(t, currentVideo, uint32(500000))

	// Give callbacks time to execute
	time.Sleep(10 * time.Millisecond)
	assert.True(t, audioCbCalled)
	assert.True(t, videoCbCalled)
	assert.Equal(t, currentAudio, capturedAudioBitRate)
	assert.Equal(t, currentVideo, capturedVideoBitRate)
}

func TestDecreaseBitrates(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	originalAudio, originalVideo := adapter.GetCurrentBitrates()
	timestamp := time.Now()

	adapter.decreaseBitrates(timestamp)

	newAudio, newVideo := adapter.GetCurrentBitrates()

	// Verify bitrates decreased according to multiplier
	expectedAudio := uint32(float64(originalAudio) * config.DecreaseMultiplier)
	expectedVideo := uint32(float64(originalVideo) * config.DecreaseMultiplier)

	assert.Equal(t, expectedAudio, newAudio)
	assert.Equal(t, expectedVideo, newVideo)
	assert.Equal(t, timestamp, adapter.lastDecrease)
}

func TestDecreaseBitratesRespectMinimums(t *testing.T) {
	config := DefaultAdaptationConfig()
	// Start with bitrates near minimum
	adapter := NewBitrateAdapter(config, config.MinAudioBitRate+1000, config.MinVideoBitRate+5000)

	timestamp := time.Now()
	adapter.decreaseBitrates(timestamp)

	audio, video := adapter.GetCurrentBitrates()

	// Should not go below minimums
	assert.GreaterOrEqual(t, audio, config.MinAudioBitRate)
	assert.GreaterOrEqual(t, video, config.MinVideoBitRate)
}

func TestIncreaseBitrates(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	originalAudio, originalVideo := adapter.GetCurrentBitrates()

	adapter.increaseBitrates()

	newAudio, newVideo := adapter.GetCurrentBitrates()

	// Verify bitrates increased according to step
	expectedAudio := uint32(float64(originalAudio) * (1.0 + config.IncreaseStep))
	expectedVideo := uint32(float64(originalVideo) * (1.0 + config.IncreaseStep))

	assert.Equal(t, expectedAudio, newAudio)
	assert.Equal(t, expectedVideo, newVideo)
}

func TestIncreaseBitratesRespectMaximums(t *testing.T) {
	config := DefaultAdaptationConfig()
	// Start with bitrates near maximum
	adapter := NewBitrateAdapter(config, config.MaxAudioBitRate-1000, config.MaxVideoBitRate-10000)

	adapter.increaseBitrates()

	audio, video := adapter.GetCurrentBitrates()

	// Should not exceed maximums
	assert.LessOrEqual(t, audio, config.MaxAudioBitRate)
	assert.LessOrEqual(t, video, config.MaxVideoBitRate)
}

func TestCanIncreaseBitrates(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	timestamp := time.Now()

	// No previous decrease - should allow increase
	assert.True(t, adapter.canIncreaseBitrates(timestamp))

	// Record a decrease
	adapter.lastDecrease = timestamp

	// Immediately after decrease - should not allow increase
	assert.False(t, adapter.canIncreaseBitrates(timestamp))

	// After backoff period - should allow increase
	futureTime := timestamp.Add(config.BackoffDuration + time.Second)
	assert.True(t, adapter.canIncreaseBitrates(futureTime))
}

func TestIsSignificantChange(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	tests := []struct {
		name        string
		oldBitRate  uint32
		newBitRate  uint32
		significant bool
	}{
		{
			name:        "no change",
			oldBitRate:  32000,
			newBitRate:  32000,
			significant: false,
		},
		{
			name:        "small change below threshold",
			oldBitRate:  32000,
			newBitRate:  35000, // 3000 change < 5000 threshold
			significant: false,
		},
		{
			name:        "significant increase",
			oldBitRate:  32000,
			newBitRate:  40000, // 8000 change > 5000 threshold
			significant: true,
		},
		{
			name:        "significant decrease",
			oldBitRate:  40000,
			newBitRate:  32000, // 8000 change > 5000 threshold
			significant: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.isSignificantChange(tt.oldBitRate, tt.newBitRate)
			assert.Equal(t, tt.significant, result)
		})
	}
}

func TestConservativeBitrates(t *testing.T) {
	config := DefaultAdaptationConfig()
	adapter := NewBitrateAdapter(config, 32000, 500000)

	originalAudio, originalVideo := adapter.GetCurrentBitrates()

	adapter.conservativeBitrates()

	newAudio, newVideo := adapter.GetCurrentBitrates()

	// Audio should remain unchanged
	assert.Equal(t, originalAudio, newAudio)

	// Video should decrease by 5%
	expectedVideo := uint32(float64(originalVideo) * 0.95)
	assert.Equal(t, expectedVideo, newVideo)
}

func TestConservativeBitratesRespectMinimum(t *testing.T) {
	config := DefaultAdaptationConfig()
	// Start with video bitrate near minimum
	adapter := NewBitrateAdapter(config, 32000, config.MinVideoBitRate+1000)

	adapter.conservativeBitrates()

	_, video := adapter.GetCurrentBitrates()

	// Should not go below minimum
	assert.GreaterOrEqual(t, video, config.MinVideoBitRate)
}

func TestGetCurrentBitrates(t *testing.T) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	audio, video := adapter.GetCurrentBitrates()
	assert.Equal(t, uint32(32000), audio)
	assert.Equal(t, uint32(500000), video)
}

func TestGetNetworkQuality(t *testing.T) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	// Initial quality should be good
	quality := adapter.GetNetworkQuality()
	assert.Equal(t, NetworkGood, quality)

	// Change quality
	adapter.currentQuality = NetworkPoor
	quality = adapter.GetNetworkQuality()
	assert.Equal(t, NetworkPoor, quality)
}

func TestGetAdaptationStats(t *testing.T) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	// Initial stats
	count, lastTime := adapter.GetAdaptationStats()
	assert.Equal(t, uint64(0), count)
	assert.False(t, lastTime.IsZero()) // Should have initial time

	// Update stats
	adapter.adaptationCount = 5
	testTime := time.Now()
	adapter.lastAdaptation = testTime

	count, lastTime = adapter.GetAdaptationStats()
	assert.Equal(t, uint64(5), count)
	assert.Equal(t, testTime, lastTime)
}

func TestAdaptationScenarioExcellentToPoor(t *testing.T) {
	config := DefaultAdaptationConfig()
	config.AdaptationWindow = 100 * time.Millisecond // Short window for testing
	adapter := NewBitrateAdapter(config, 32000, 500000)

	timestamp := time.Now()

	// Start with excellent conditions
	_, err := adapter.UpdateNetworkStats(100, 100, 0, 20*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.Equal(t, NetworkExcellent, adapter.GetNetworkQuality())

	// Wait for adaptation window
	time.Sleep(150 * time.Millisecond)
	timestamp = time.Now()

	// Network degrades to poor conditions
	adapted, err := adapter.UpdateNetworkStats(100, 85, 15, 200*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.True(t, adapted)
	assert.Equal(t, NetworkPoor, adapter.GetNetworkQuality())

	// Verify bitrates decreased significantly
	audio, video := adapter.GetCurrentBitrates()
	assert.Less(t, audio, uint32(32000))
	assert.Less(t, video, uint32(500000))
}

func TestAdaptationScenarioPoorToGood(t *testing.T) {
	config := DefaultAdaptationConfig()
	config.AdaptationWindow = 100 * time.Millisecond
	config.BackoffDuration = 50 * time.Millisecond // Short backoff for testing
	adapter := NewBitrateAdapter(config, 32000, 500000)

	timestamp := time.Now()

	// Start with poor conditions and force adaptation
	_, err := adapter.UpdateNetworkStats(100, 85, 15, 200*time.Millisecond, timestamp)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
	timestamp = time.Now()

	// Trigger poor quality adaptation
	adapted, err := adapter.UpdateNetworkStats(100, 85, 15, 200*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.True(t, adapted)

	// Get bitrates after decrease
	audioAfterDecrease, videoAfterDecrease := adapter.GetCurrentBitrates()

	// Wait for backoff duration
	time.Sleep(100 * time.Millisecond)
	timestamp = time.Now()

	// Network improves to excellent conditions (< 1% loss, < 50ms jitter)
	time.Sleep(150 * time.Millisecond)
	timestamp = time.Now()

	adapted, err = adapter.UpdateNetworkStats(100, 100, 0, 30*time.Millisecond, timestamp)
	require.NoError(t, err)
	assert.True(t, adapted)
	assert.Equal(t, NetworkExcellent, adapter.GetNetworkQuality())

	// Verify bitrates increased
	audioAfterIncrease, videoAfterIncrease := adapter.GetCurrentBitrates()
	assert.Greater(t, audioAfterIncrease, audioAfterDecrease)
	assert.Greater(t, videoAfterIncrease, videoAfterDecrease)
}

// Benchmark bitrate adaptation performance
func BenchmarkUpdateNetworkStats(b *testing.B) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.UpdateNetworkStats(100, 95, 5, 80*time.Millisecond, timestamp)
		timestamp = timestamp.Add(time.Second)
	}
}

func BenchmarkAssessNetworkQuality(b *testing.B) {
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 32000, 500000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.assessNetworkQuality(3.0, 80*time.Millisecond)
	}
}
