package av

import (
	"testing"
	"time"
)

// TestDefaultQualityThresholds verifies default quality thresholds are reasonable.
func TestDefaultQualityThresholds(t *testing.T) {
	thresholds := DefaultQualityThresholds()

	if thresholds == nil {
		t.Fatal("DefaultQualityThresholds returned nil")
	}

	// Verify packet loss thresholds are in ascending order
	if thresholds.ExcellentPacketLoss >= thresholds.GoodPacketLoss {
		t.Error("Excellent packet loss threshold should be less than good")
	}
	if thresholds.GoodPacketLoss >= thresholds.FairPacketLoss {
		t.Error("Good packet loss threshold should be less than fair")
	}
	if thresholds.FairPacketLoss >= thresholds.PoorPacketLoss {
		t.Error("Fair packet loss threshold should be less than poor")
	}

	// Verify jitter thresholds are in ascending order
	if thresholds.ExcellentJitter >= thresholds.GoodJitter {
		t.Error("Excellent jitter threshold should be less than good")
	}
	if thresholds.GoodJitter >= thresholds.FairJitter {
		t.Error("Good jitter threshold should be less than fair")
	}
	if thresholds.FairJitter >= thresholds.PoorJitter {
		t.Error("Fair jitter threshold should be less than poor")
	}

	// Verify frame timeout is reasonable
	if thresholds.FrameTimeout <= 0 {
		t.Error("Frame timeout should be positive")
	}
	if thresholds.FrameTimeout < time.Second {
		t.Error("Frame timeout should be at least 1 second")
	}
}

// TestQualityLevelString verifies string representation of quality levels.
func TestQualityLevelString(t *testing.T) {
	tests := []struct {
		level    QualityLevel
		expected string
	}{
		{QualityExcellent, "Excellent"},
		{QualityGood, "Good"},
		{QualityFair, "Fair"},
		{QualityPoor, "Poor"},
		{QualityUnacceptable, "Unacceptable"},
		{QualityLevel(999), "Unknown(999)"},
	}

	for _, test := range tests {
		result := test.level.String()
		if result != test.expected {
			t.Errorf("Expected %s, got %s for level %d", test.expected, result, int(test.level))
		}
	}
}

// TestNewQualityMonitor verifies quality monitor creation.
func TestNewQualityMonitor(t *testing.T) {
	// Test with default thresholds
	monitor := NewQualityMonitor(nil)
	if monitor == nil {
		t.Fatal("NewQualityMonitor returned nil")
	}

	if !monitor.IsEnabled() {
		t.Error("Monitor should be enabled by default")
	}

	if monitor.GetMonitorInterval() <= 0 {
		t.Error("Monitor interval should be positive")
	}

	// Test with custom thresholds
	customThresholds := &QualityThresholds{
		ExcellentPacketLoss: 0.5,
		GoodPacketLoss:      2.0,
		FairPacketLoss:      5.0,
		PoorPacketLoss:      10.0,
		ExcellentJitter:     10 * time.Millisecond,
		GoodJitter:          30 * time.Millisecond,
		FairJitter:          60 * time.Millisecond,
		PoorJitter:          120 * time.Millisecond,
		FrameTimeout:        3 * time.Second,
	}

	monitor = NewQualityMonitor(customThresholds)
	if monitor == nil {
		t.Fatal("NewQualityMonitor with custom thresholds returned nil")
	}
}

// TestQualityMonitorConfiguration verifies monitor configuration methods.
func TestQualityMonitorConfiguration(t *testing.T) {
	monitor := NewQualityMonitor(nil)

	// Test enabled/disabled state
	monitor.SetEnabled(false)
	if monitor.IsEnabled() {
		t.Error("Monitor should be disabled after SetEnabled(false)")
	}

	monitor.SetEnabled(true)
	if !monitor.IsEnabled() {
		t.Error("Monitor should be enabled after SetEnabled(true)")
	}

	// Test monitor interval
	newInterval := 10 * time.Second
	monitor.SetMonitorInterval(newInterval)
	if monitor.GetMonitorInterval() != newInterval {
		t.Errorf("Expected interval %v, got %v", newInterval, monitor.GetMonitorInterval())
	}

	// Test quality callback
	callback := func(friendNumber uint32, metrics CallMetrics) {
		// Callback would be tested in monitoring integration
	}

	monitor.SetQualityCallback(callback)
	// Note: callback testing requires actual monitoring which is tested separately

	monitor.SetQualityCallback(nil) // Should not panic
}

// TestQualityAssessment verifies quality level assessment logic.
func TestQualityAssessment(t *testing.T) {
	monitor := NewQualityMonitor(nil)

	tests := []struct {
		name            string
		metrics         CallMetrics
		expectedQuality QualityLevel
	}{
		{
			name: "excellent_quality",
			metrics: CallMetrics{
				PacketLoss:   0.5,                   // < 1.0%
				Jitter:       15 * time.Millisecond, // < 20ms
				LastFrameAge: 100 * time.Millisecond,
			},
			expectedQuality: QualityExcellent,
		},
		{
			name: "good_quality_low_jitter",
			metrics: CallMetrics{
				PacketLoss:   0.8,                   // < 1.0%
				Jitter:       25 * time.Millisecond, // > 20ms but < 50ms
				LastFrameAge: 200 * time.Millisecond,
			},
			expectedQuality: QualityGood,
		},
		{
			name: "fair_quality_packet_loss",
			metrics: CallMetrics{
				PacketLoss:   5.0, // > 3.0% but < 8.0%
				Jitter:       30 * time.Millisecond,
				LastFrameAge: 300 * time.Millisecond,
			},
			expectedQuality: QualityFair,
		},
		{
			name: "poor_quality_high_packet_loss",
			metrics: CallMetrics{
				PacketLoss:   10.0, // > 8.0% but < 15.0%
				Jitter:       40 * time.Millisecond,
				LastFrameAge: 500 * time.Millisecond,
			},
			expectedQuality: QualityPoor,
		},
		{
			name: "unacceptable_packet_loss",
			metrics: CallMetrics{
				PacketLoss:   20.0, // > 15.0%
				Jitter:       50 * time.Millisecond,
				LastFrameAge: 800 * time.Millisecond,
			},
			expectedQuality: QualityUnacceptable,
		},
		{
			name: "unacceptable_frame_timeout",
			metrics: CallMetrics{
				PacketLoss:   0.5,                   // Excellent packet loss
				Jitter:       15 * time.Millisecond, // Excellent jitter
				LastFrameAge: 3 * time.Second,       // > 2 second timeout
			},
			expectedQuality: QualityUnacceptable,
		},
		{
			name: "good_quality_moderate_jitter",
			metrics: CallMetrics{
				PacketLoss:   0.5,                    // Excellent packet loss
				Jitter:       150 * time.Millisecond, // Between FairJitter(100ms) and PoorJitter(200ms)
				LastFrameAge: 100 * time.Millisecond,
			},
			expectedQuality: QualityGood, // With excellent packet loss, moderate jitter gives Good
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quality := monitor.assessQuality(test.metrics)
			if quality != test.expectedQuality {
				t.Errorf("Expected quality %s, got %s for test %s",
					test.expectedQuality.String(), quality.String(), test.name)
			}
		})
	}
}

// TestGetCallMetrics verifies metrics collection from calls.
func TestGetCallMetrics(t *testing.T) {
	monitor := NewQualityMonitor(nil)

	// Test with nil call
	_, err := monitor.GetCallMetrics(nil, nil)
	if err == nil {
		t.Error("Expected error for nil call")
	}

	// Create a test call
	call := NewCall(123)

	// Set up call timing
	call.markStarted()
	time.Sleep(10 * time.Millisecond) // Small delay for measurable duration
	call.updateLastFrame()

	// Set bitrates
	call.SetAudioBitRate(64000)
	call.SetVideoBitRate(1000000)

	// Test without adapter (should work but show poor network quality)
	metrics, err := monitor.GetCallMetrics(call, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify basic metrics
	if metrics.AudioBitRate != 64000 {
		t.Errorf("Expected audio bitrate 64000, got %d", metrics.AudioBitRate)
	}
	if metrics.VideoBitRate != 1000000 {
		t.Errorf("Expected video bitrate 1000000, got %d", metrics.VideoBitRate)
	}
	if metrics.CallDuration <= 0 {
		t.Error("Call duration should be positive")
	}
	if metrics.LastFrameAge < 0 {
		t.Error("Last frame age should not be negative")
	}
	if metrics.NetworkQuality != NetworkPoor {
		t.Errorf("Expected NetworkPoor without adapter, got %v", metrics.NetworkQuality)
	}
	if metrics.Timestamp.IsZero() {
		t.Error("Metrics timestamp should be set")
	}

	// Verify quality is assessed
	if metrics.Quality == QualityLevel(-1) {
		t.Error("Quality should be assessed")
	}
}

// TestGetCallMetricsWithAdapter verifies metrics collection with bitrate adapter.
func TestGetCallMetricsWithAdapter(t *testing.T) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(456)
	call.markStarted()
	call.updateLastFrame()

	// Create adapter with good network quality
	adapter := NewBitrateAdapter(DefaultAdaptationConfig(), 64000, 1000000)
	// Simulate good network quality by calling methods that would set it
	// (In real usage, adapter would be updated by network measurements)

	metrics, err := monitor.GetCallMetrics(call, adapter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have network quality from adapter
	networkQuality := adapter.GetNetworkQuality()
	if metrics.NetworkQuality != networkQuality {
		t.Errorf("Expected network quality %v, got %v", networkQuality, metrics.NetworkQuality)
	}
}

// TestMonitorCall verifies call monitoring functionality.
func TestMonitorCall(t *testing.T) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(789)
	call.markStarted()
	call.updateLastFrame()

	// Test monitoring when enabled
	metrics, err := monitor.MonitorCall(call, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return valid metrics
	if metrics.Timestamp.IsZero() {
		t.Error("Metrics should have valid timestamp when monitoring enabled")
	}

	// Test monitoring when disabled
	monitor.SetEnabled(false)
	metrics, err = monitor.MonitorCall(call, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return empty metrics when disabled
	if !metrics.Timestamp.IsZero() {
		t.Error("Metrics should be empty when monitoring disabled")
	}
}

// TestQualityCallback verifies quality change callbacks.
func TestQualityCallback(t *testing.T) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(101112)
	call.markStarted()
	call.updateLastFrame()

	// Set up callback
	var callbackFriendNumber uint32
	var callbackMetrics CallMetrics
	callbackCalled := false

	monitor.SetQualityCallback(func(friendNumber uint32, metrics CallMetrics) {
		callbackFriendNumber = friendNumber
		callbackMetrics = metrics
		callbackCalled = true
	})

	// Monitor call - should trigger callback
	_, err := monitor.MonitorCall(call, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify callback was called
	if !callbackCalled {
		t.Error("Quality callback should have been called")
	}
	if callbackFriendNumber != call.GetFriendNumber() {
		t.Errorf("Expected friend number %d, got %d", call.GetFriendNumber(), callbackFriendNumber)
	}
	if callbackMetrics.Timestamp.IsZero() {
		t.Error("Callback metrics should have valid timestamp")
	}

	// Test monitoring without callback
	monitor.SetQualityCallback(nil)
	// Should not panic when monitoring without callback
	_, err = monitor.MonitorCall(call, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// TestQualityMonitorThreadSafety verifies thread safety of quality monitor.
func TestQualityMonitorThreadSafety(t *testing.T) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(131415)
	call.markStarted()
	call.updateLastFrame()

	// Run concurrent operations
	done := make(chan bool, 4)

	// Concurrent enabled/disabled changes
	go func() {
		for i := 0; i < 10; i++ {
			monitor.SetEnabled(i%2 == 0)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent interval changes
	go func() {
		for i := 0; i < 10; i++ {
			monitor.SetMonitorInterval(time.Duration(i+1) * time.Second)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent monitoring
	go func() {
		for i := 0; i < 10; i++ {
			_, _ = monitor.MonitorCall(call, nil)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent callback changes
	go func() {
		for i := 0; i < 10; i++ {
			if i%2 == 0 {
				monitor.SetQualityCallback(func(uint32, CallMetrics) {})
			} else {
				monitor.SetQualityCallback(nil)
			}
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// Final verification - should not panic
	_ = monitor.IsEnabled()
	_ = monitor.GetMonitorInterval()
}

// BenchmarkGetCallMetrics benchmarks metrics collection performance.
func BenchmarkGetCallMetrics(b *testing.B) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(161718)
	call.markStarted()
	call.updateLastFrame()
	call.SetAudioBitRate(64000)
	call.SetVideoBitRate(1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := monitor.GetCallMetrics(call, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkQualityAssessment benchmarks quality assessment performance.
func BenchmarkQualityAssessment(b *testing.B) {
	monitor := NewQualityMonitor(nil)
	metrics := CallMetrics{
		PacketLoss:   2.5,
		Jitter:       35 * time.Millisecond,
		LastFrameAge: 150 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.assessQuality(metrics)
	}
}

// BenchmarkMonitorCall benchmarks call monitoring performance.
func BenchmarkMonitorCall(b *testing.B) {
	monitor := NewQualityMonitor(nil)
	call := NewCall(192021)
	call.markStarted()
	call.updateLastFrame()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := monitor.MonitorCall(call, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
