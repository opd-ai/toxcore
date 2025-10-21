package av

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMetricsAggregator verifies basic aggregator creation.
func TestNewMetricsAggregator(t *testing.T) {
	reportInterval := 3 * time.Second
	aggregator := NewMetricsAggregator(reportInterval)

	require.NotNil(t, aggregator)
	assert.Equal(t, reportInterval, aggregator.reportInterval)
	assert.False(t, aggregator.IsRunning())
	assert.Equal(t, 0, aggregator.GetActiveCallCount())
	assert.Equal(t, uint64(0), aggregator.GetTotalCallCount())
}

// TestAggregatorStartStop verifies aggregator lifecycle.
func TestAggregatorStartStop(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	// Initially not running
	assert.False(t, aggregator.IsRunning())

	// Start aggregator
	err := aggregator.Start()
	assert.NoError(t, err)
	assert.True(t, aggregator.IsRunning())

	// Starting again should fail
	err = aggregator.Start()
	assert.ErrorIs(t, err, ErrAlreadyRunning)

	// Stop aggregator
	aggregator.Stop()
	assert.False(t, aggregator.IsRunning())

	// Stopping again should be safe
	aggregator.Stop()
	assert.False(t, aggregator.IsRunning())
}

// TestStartCallTracking verifies call tracking initialization.
func TestStartCallTracking(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	friendNumber := uint32(42)

	// Track call
	aggregator.StartCallTracking(friendNumber)

	// Verify counts updated
	assert.Equal(t, 1, aggregator.GetActiveCallCount())
	assert.Equal(t, uint64(1), aggregator.GetTotalCallCount())

	// System metrics should reflect active call
	systemMetrics := aggregator.GetSystemMetrics()
	assert.Equal(t, 1, systemMetrics.ActiveCalls)
	assert.Equal(t, uint64(1), systemMetrics.TotalCalls)
}

// TestStopCallTracking verifies call tracking cleanup.
func TestStopCallTracking(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	friendNumber := uint32(42)

	// Start and stop tracking
	aggregator.StartCallTracking(friendNumber)
	assert.Equal(t, 1, aggregator.GetActiveCallCount())

	aggregator.StopCallTracking(friendNumber)
	assert.Equal(t, 0, aggregator.GetActiveCallCount())

	// Total calls should still be 1
	assert.Equal(t, uint64(1), aggregator.GetTotalCallCount())
}

// TestRecordMetrics verifies metrics recording and history.
func TestRecordMetrics(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	friendNumber := uint32(42)
	aggregator.StartCallTracking(friendNumber)

	// Record some metrics
	metrics := CallMetrics{
		PacketLoss:      2.5,
		Jitter:          30 * time.Millisecond,
		RoundTripTime:   50 * time.Millisecond,
		PacketsSent:     1000,
		PacketsReceived: 975,
		AudioBitRate:    64000,
		VideoBitRate:    1000000,
		NetworkQuality:  NetworkGood,
		CallDuration:    2 * time.Minute,
		LastFrameAge:    100 * time.Millisecond,
		Quality:         QualityGood,
		Timestamp:       time.Now(),
	}

	aggregator.RecordMetrics(friendNumber, metrics)

	// Verify history
	history := aggregator.GetCallHistory(friendNumber)
	require.NotNil(t, history)
	require.Len(t, history, 1)
	assert.Equal(t, metrics.PacketLoss, history[0].PacketLoss)
	assert.Equal(t, metrics.Quality, history[0].Quality)
}

// TestMetricsHistory verifies rolling window history management.
func TestMetricsHistory(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	friendNumber := uint32(42)
	aggregator.StartCallTracking(friendNumber)

	// Record metrics multiple times (more than max history)
	for i := 0; i < 70; i++ {
		metrics := CallMetrics{
			PacketLoss:  float64(i),
			Quality:     QualityGood,
			Timestamp:   time.Now(),
		}
		aggregator.RecordMetrics(friendNumber, metrics)
	}

	// History should be capped at MaxHistory (60)
	history := aggregator.GetCallHistory(friendNumber)
	require.NotNil(t, history)
	assert.Equal(t, 60, len(history))

	// Oldest entries should be removed (should start from 10)
	assert.Equal(t, 10.0, history[0].PacketLoss)
	assert.Equal(t, 69.0, history[len(history)-1].PacketLoss)
}

// TestSystemMetricsAggregation verifies system-wide metric calculation.
func TestSystemMetricsAggregation(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	// Track multiple calls with different qualities
	calls := []struct {
		friendNumber uint32
		quality      QualityLevel
		packetLoss   float64
		jitter       time.Duration
		bitrate      uint32
	}{
		{42, QualityExcellent, 0.5, 15 * time.Millisecond, 128000},
		{43, QualityGood, 2.0, 35 * time.Millisecond, 256000},
		{44, QualityFair, 5.0, 75 * time.Millisecond, 192000},
		{45, QualityPoor, 12.0, 150 * time.Millisecond, 64000},
	}

	for _, call := range calls {
		aggregator.StartCallTracking(call.friendNumber)
		metrics := CallMetrics{
			PacketLoss:    call.packetLoss,
			Jitter:        call.jitter,
			AudioBitRate:  call.bitrate / 2,
			VideoBitRate:  call.bitrate / 2,
			Quality:       call.quality,
			CallDuration:  2 * time.Minute,
			Timestamp:     time.Now(),
		}
		aggregator.RecordMetrics(call.friendNumber, metrics)
	}

	// Check system metrics
	systemMetrics := aggregator.GetSystemMetrics()
	assert.Equal(t, 4, systemMetrics.ActiveCalls)
	assert.Equal(t, 1, systemMetrics.ExcellentCalls)
	assert.Equal(t, 1, systemMetrics.GoodCalls)
	assert.Equal(t, 1, systemMetrics.FairCalls)
	assert.Equal(t, 1, systemMetrics.PoorCalls)

	// Verify averages
	expectedAvgLoss := (0.5 + 2.0 + 5.0 + 12.0) / 4.0
	assert.InDelta(t, expectedAvgLoss, systemMetrics.AveragePacketLoss, 0.01)

	expectedAvgBitrate := uint32((128000 + 256000 + 192000 + 64000) / 4)
	assert.Equal(t, expectedAvgBitrate, systemMetrics.AverageBitrate)
}

// TestOverallQualityCalculation verifies system-wide quality assessment.
func TestOverallQualityCalculation(t *testing.T) {
	tests := []struct {
		name              string
		qualities         []QualityLevel
		expectedOverall   QualityLevel
	}{
		{
			name:            "all_excellent",
			qualities:       []QualityLevel{QualityExcellent, QualityExcellent, QualityExcellent},
			expectedOverall: QualityExcellent,
		},
		{
			name:            "mostly_good",
			qualities:       []QualityLevel{QualityGood, QualityGood, QualityFair},
			expectedOverall: QualityGood,
		},
		{
			name:            "mostly_fair",
			qualities:       []QualityLevel{QualityFair, QualityFair, QualityGood},
			expectedOverall: QualityFair,
		},
		{
			name:            "mostly_poor",
			qualities:       []QualityLevel{QualityPoor, QualityPoor, QualityGood},
			expectedOverall: QualityPoor,
		},
		{
			name:            "mixed_good_and_excellent",
			qualities:       []QualityLevel{QualityExcellent, QualityExcellent, QualityGood, QualityGood},
			expectedOverall: QualityExcellent, // More excellent than good
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewMetricsAggregator(1 * time.Second)

			// Track calls with specified qualities
			for i, quality := range tt.qualities {
				friendNumber := uint32(100 + i)
				aggregator.StartCallTracking(friendNumber)
				metrics := CallMetrics{
					Quality:   quality,
					Timestamp: time.Now(),
				}
				aggregator.RecordMetrics(friendNumber, metrics)
			}

			// Calculate overall quality
			aggregator.mu.Lock()
			overall := aggregator.calculateOverallQuality()
			aggregator.mu.Unlock()

			assert.Equal(t, tt.expectedOverall, overall)
		})
	}
}

// TestReportCallback verifies periodic report generation.
func TestReportCallback(t *testing.T) {
	reportInterval := 100 * time.Millisecond
	aggregator := NewMetricsAggregator(reportInterval)
	require.NotNil(t, aggregator)

	// Set up report callback
	var reportReceived sync.WaitGroup
	reportReceived.Add(1)
	var capturedReport AggregatedReport

	aggregator.OnReport(func(report AggregatedReport) {
		capturedReport = report
		reportReceived.Done()
	})

	// Start aggregator
	err := aggregator.Start()
	require.NoError(t, err)
	defer aggregator.Stop()

	// Track a call and record metrics
	friendNumber := uint32(42)
	aggregator.StartCallTracking(friendNumber)
	metrics := CallMetrics{
		PacketLoss:   2.0,
		Quality:      QualityGood,
		AudioBitRate: 64000,
		CallDuration: 1 * time.Minute,
		Timestamp:    time.Now(),
	}
	aggregator.RecordMetrics(friendNumber, metrics)

	// Wait for report with timeout
	done := make(chan struct{})
	go func() {
		reportReceived.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify report contents
		assert.Equal(t, 1, capturedReport.SystemMetrics.ActiveCalls)
		assert.Contains(t, capturedReport.CallReports, friendNumber)
		assert.Equal(t, reportInterval, capturedReport.ReportDuration)
		assert.NotZero(t, capturedReport.Timestamp)
	case <-time.After(1 * time.Second):
		t.Fatal("Report callback not called within timeout")
	}
}

// TestConcurrentMetricsRecording verifies thread safety.
func TestConcurrentMetricsRecording(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	friendNumbers := []uint32{42, 43, 44, 45}

	// Start tracking all calls
	for _, fn := range friendNumbers {
		aggregator.StartCallTracking(fn)
	}

	// Concurrently record metrics for different calls
	var wg sync.WaitGroup
	iterations := 50

	for _, fn := range friendNumbers {
		wg.Add(1)
		go func(friendNumber uint32) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				metrics := CallMetrics{
					PacketLoss:  float64(i % 10),
					Quality:     QualityGood,
					AudioBitRate: 64000,
					Timestamp:   time.Now(),
				}
				aggregator.RecordMetrics(friendNumber, metrics)
				time.Sleep(time.Microsecond)
			}
		}(fn)
	}

	// Concurrent reads
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations*2; i++ {
			_ = aggregator.GetSystemMetrics()
			for _, fn := range friendNumbers {
				_ = aggregator.GetCallHistory(fn)
			}
			time.Sleep(time.Microsecond)
		}
	}()

	// Wait for all goroutines
	wg.Wait()

	// Verify final state
	assert.Equal(t, len(friendNumbers), aggregator.GetActiveCallCount())
	systemMetrics := aggregator.GetSystemMetrics()
	assert.Equal(t, len(friendNumbers), systemMetrics.ActiveCalls)
}

// TestMultipleCallTracking verifies tracking multiple calls simultaneously.
func TestMultipleCallTracking(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	// Track multiple calls
	friendNumbers := []uint32{42, 43, 44}
	for _, fn := range friendNumbers {
		aggregator.StartCallTracking(fn)
	}

	assert.Equal(t, len(friendNumbers), aggregator.GetActiveCallCount())
	assert.Equal(t, uint64(len(friendNumbers)), aggregator.GetTotalCallCount())

	// Stop one call
	aggregator.StopCallTracking(friendNumbers[1])
	assert.Equal(t, 2, aggregator.GetActiveCallCount())
	assert.Equal(t, uint64(3), aggregator.GetTotalCallCount())

	// Stop remaining calls
	for _, fn := range []uint32{friendNumbers[0], friendNumbers[2]} {
		aggregator.StopCallTracking(fn)
	}

	assert.Equal(t, 0, aggregator.GetActiveCallCount())
	assert.Equal(t, uint64(3), aggregator.GetTotalCallCount())
}

// TestGetCallHistoryNonExistent verifies behavior for non-tracked calls.
func TestGetCallHistoryNonExistent(t *testing.T) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	require.NotNil(t, aggregator)

	// Get history for non-existent call
	history := aggregator.GetCallHistory(999)
	assert.Nil(t, history)
}

// BenchmarkRecordMetrics measures performance of metrics recording.
func BenchmarkRecordMetrics(b *testing.B) {
	aggregator := NewMetricsAggregator(1 * time.Second)
	friendNumber := uint32(42)
	aggregator.StartCallTracking(friendNumber)

	metrics := CallMetrics{
		PacketLoss:   2.0,
		Quality:      QualityGood,
		AudioBitRate: 64000,
		Timestamp:    time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregator.RecordMetrics(friendNumber, metrics)
	}
}

// BenchmarkGetSystemMetrics measures performance of system metrics retrieval.
func BenchmarkGetSystemMetrics(b *testing.B) {
	aggregator := NewMetricsAggregator(1 * time.Second)

	// Track some calls
	for i := uint32(0); i < 10; i++ {
		aggregator.StartCallTracking(i)
		metrics := CallMetrics{
			PacketLoss:   2.0,
			Quality:      QualityGood,
			AudioBitRate: 64000,
			Timestamp:    time.Now(),
		}
		aggregator.RecordMetrics(i, metrics)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aggregator.GetSystemMetrics()
	}
}
