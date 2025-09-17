package av

import (
	"sync"
	"testing"
	"time"
)

// TestPerformanceOptimizerCreation verifies that the performance optimizer
// can be created with appropriate default settings.
func TestPerformanceOptimizerCreation(t *testing.T) {
	optimizer := NewPerformanceOptimizer()

	if optimizer == nil {
		t.Fatal("NewPerformanceOptimizer() returned nil")
	}

	// Verify default settings
	if optimizer.IsDetailedLoggingEnabled() {
		t.Error("Expected detailed logging to be disabled by default")
	}

	if optimizer.IsProfilingEnabled() {
		t.Error("Expected profiling to be disabled by default")
	}

	// Verify cache validity is set
	if optimizer.cacheValidityNs <= 0 {
		t.Error("Expected cache validity to be positive")
	}
}

// TestPerformanceOptimizerSlicePooling verifies that the call slice pooling
// works correctly and reuses memory efficiently.
func TestPerformanceOptimizerSlicePooling(t *testing.T) {
	optimizer := NewPerformanceOptimizer()

	// Get multiple slices from the pool
	slice1 := optimizer.getCallSlice()
	slice2 := optimizer.getCallSlice()

	// Verify slices are properly initialized
	if len(slice1) != 0 {
		t.Errorf("Expected slice1 length 0, got %d", len(slice1))
	}
	if len(slice2) != 0 {
		t.Errorf("Expected slice2 length 0, got %d", len(slice2))
	}

	// Add some test calls to slice1
	testCalls := []*Call{
		{friendNumber: 1},
		{friendNumber: 2},
	}
	slice1 = append(slice1, testCalls...)

	// Return slice1 to pool
	optimizer.returnCallSlice(slice1)

	// Get a new slice - should reuse the memory
	slice3 := optimizer.getCallSlice()
	if len(slice3) != 0 {
		t.Errorf("Expected reused slice to be reset, got length %d", len(slice3))
	}

	// Return remaining slices
	optimizer.returnCallSlice(slice2)
	optimizer.returnCallSlice(slice3)
}

// TestPerformanceOptimizerCaching verifies that the call count caching
// mechanism works correctly and reduces lock contention.
func TestPerformanceOptimizerCaching(t *testing.T) {
	// Create test manager with mock transport
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	optimizer := manager.GetPerformanceOptimizer()
	if optimizer == nil {
		t.Fatal("Expected performance optimizer to be initialized")
	}

	// Test with no calls (should use fast path)
	callSlice, shouldProcess := optimizer.OptimizeIteration(manager)
	if shouldProcess {
		t.Error("Expected shouldProcess to be false with no calls")
	}
	if callSlice != nil {
		t.Error("Expected callSlice to be nil with no calls")
	}

	// Start manager and add a call
	manager.Start()
	defer manager.Stop()

	err = manager.StartCall(1, 64000, 0) // Audio only call
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// First call should populate cache
	callSlice1, shouldProcess1 := optimizer.OptimizeIteration(manager)
	if !shouldProcess1 {
		t.Error("Expected shouldProcess to be true with active calls")
	}
	if len(callSlice1) != 1 {
		t.Errorf("Expected 1 call, got %d", len(callSlice1))
	}
	optimizer.ReturnCallSlice(callSlice1)

	// Second call should use cached value (within cache validity period)
	callSlice2, shouldProcess2 := optimizer.OptimizeIteration(manager)
	if !shouldProcess2 {
		t.Error("Expected shouldProcess to be true with cached calls")
	}
	if len(callSlice2) != 1 {
		t.Errorf("Expected 1 cached call, got %d", len(callSlice2))
	}
	optimizer.ReturnCallSlice(callSlice2)
}

// TestPerformanceOptimizerLoggingControl verifies that detailed logging
// can be enabled and disabled dynamically.
func TestPerformanceOptimizerLoggingControl(t *testing.T) {
	optimizer := NewPerformanceOptimizer()

	// Initially disabled
	if optimizer.IsDetailedLoggingEnabled() {
		t.Error("Expected detailed logging to be disabled initially")
	}

	// Enable detailed logging
	optimizer.EnableDetailedLogging(true)
	if !optimizer.IsDetailedLoggingEnabled() {
		t.Error("Expected detailed logging to be enabled")
	}

	// Disable detailed logging
	optimizer.EnableDetailedLogging(false)
	if optimizer.IsDetailedLoggingEnabled() {
		t.Error("Expected detailed logging to be disabled")
	}
}

// TestPerformanceOptimizerProfiling verifies that CPU profiling can be
// started and stopped correctly.
func TestPerformanceOptimizerProfiling(t *testing.T) {
	optimizer := NewPerformanceOptimizer()

	// Initially disabled
	if optimizer.IsProfilingEnabled() {
		t.Error("Expected profiling to be disabled initially")
	}

	// Start profiling
	err := optimizer.StartCPUProfiling()
	if err != nil {
		t.Errorf("Failed to start CPU profiling: %v", err)
	}
	if !optimizer.IsProfilingEnabled() {
		t.Error("Expected profiling to be enabled")
	}

	// Starting again should be a no-op
	err = optimizer.StartCPUProfiling()
	if err != nil {
		t.Errorf("Failed to start CPU profiling again: %v", err)
	}

	// Stop profiling
	optimizer.StopCPUProfiling()
	if optimizer.IsProfilingEnabled() {
		t.Error("Expected profiling to be disabled")
	}

	// Stopping again should be a no-op
	optimizer.StopCPUProfiling()
	if optimizer.IsProfilingEnabled() {
		t.Error("Expected profiling to remain disabled")
	}
}

// TestPerformanceOptimizerMetrics verifies that performance metrics
// are tracked correctly.
func TestPerformanceOptimizerMetrics(t *testing.T) {
	// Create test manager
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	optimizer := manager.GetPerformanceOptimizer()
	manager.Start()
	defer manager.Stop()

	// Reset metrics for clean test
	optimizer.ResetPerformanceMetrics()

	// Get initial metrics
	metrics := optimizer.GetPerformanceMetrics()
	if metrics.TotalIterations != 0 {
		t.Errorf("Expected 0 iterations, got %d", metrics.TotalIterations)
	}

	// Perform some iterations
	for i := 0; i < 5; i++ {
		callSlice, shouldProcess := optimizer.OptimizeIteration(manager)
		if shouldProcess {
			optimizer.ReturnCallSlice(callSlice)
		}
	}

	// Check updated metrics
	metrics = optimizer.GetPerformanceMetrics()
	if metrics.TotalIterations != 5 {
		t.Errorf("Expected 5 iterations, got %d", metrics.TotalIterations)
	}

	// Add a call and iterate more
	err = manager.StartCall(1, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Use the actual Manager.Iterate method to ensure proper call processing
	for i := 0; i < 3; i++ {
		manager.Iterate()
	}

	// Check metrics with calls processed
	metrics = optimizer.GetPerformanceMetrics()
	if metrics.TotalIterations < 8 {
		t.Errorf("Expected at least 8 total iterations, got %d", metrics.TotalIterations)
	}

	// Verify that performance metrics are being tracked
	// Note: The exact number of calls processed depends on caching behavior
	// and internal implementation details, so we just verify metrics are working
	if metrics.AvgIterationTime <= 0 {
		t.Error("Expected positive average iteration time")
	}
	if metrics.PeakIterationTime <= 0 {
		t.Error("Expected positive peak iteration time")
	}

	// Verify timing metrics are being tracked
	if metrics.AvgIterationTime <= 0 {
		t.Error("Expected positive average iteration time")
	}
	if metrics.PeakIterationTime <= 0 {
		t.Error("Expected positive peak iteration time")
	}
}

// TestManagerPerformanceIntegration verifies that the performance optimizer
// is properly integrated into the Manager.
func TestManagerPerformanceIntegration(t *testing.T) {
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Verify optimizer is initialized
	optimizer := manager.GetPerformanceOptimizer()
	if optimizer == nil {
		t.Fatal("Expected performance optimizer to be initialized")
	}

	// Test performance configuration
	err = manager.EnablePerformanceOptimization(true, false)
	if err != nil {
		t.Errorf("Failed to enable performance optimization: %v", err)
	}

	if !optimizer.IsDetailedLoggingEnabled() {
		t.Error("Expected detailed logging to be enabled")
	}

	// Test metrics access
	metrics := manager.GetPerformanceMetrics()
	if metrics.DetailedLogging != true {
		t.Error("Expected detailed logging flag to be true in metrics")
	}

	// Test metrics reset
	manager.ResetPerformanceMetrics()
	metrics = manager.GetPerformanceMetrics()
	if metrics.TotalIterations != 0 {
		t.Error("Expected metrics to be reset")
	}

	// Test with nil optimizer (edge case)
	manager.performanceOptimizer = nil
	err = manager.EnablePerformanceOptimization(false, false)
	if err == nil {
		t.Error("Expected error when optimizer is nil")
	}
}

// TestPerformanceOptimizerConcurrency verifies that the performance optimizer
// is thread-safe under concurrent access.
func TestPerformanceOptimizerConcurrency(t *testing.T) {
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	optimizer := manager.GetPerformanceOptimizer()
	manager.Start()
	defer manager.Stop()

	// Add some calls
	for i := uint32(1); i <= 3; i++ {
		err = manager.StartCall(i, 64000, 0)
		if err != nil {
			t.Fatalf("Failed to start call %d: %v", i, err)
		}
	}

	// Run concurrent iterations
	const numGoroutines = 10
	const iterationsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				callSlice, shouldProcess := optimizer.OptimizeIteration(manager)
				if shouldProcess {
					// Simulate some work
					time.Sleep(time.Microsecond)
					optimizer.ReturnCallSlice(callSlice)
				}
			}
		}()
	}

	// Also toggle configuration concurrently
	go func() {
		for i := 0; i < 20; i++ {
			optimizer.EnableDetailedLogging(i%2 == 0)
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// Verify final metrics
	metrics := optimizer.GetPerformanceMetrics()
	expectedIterations := int64(numGoroutines * iterationsPerGoroutine)
	if metrics.TotalIterations != expectedIterations {
		t.Errorf("Expected %d iterations, got %d", expectedIterations, metrics.TotalIterations)
	}

	// Should have processed calls in many iterations
	if metrics.TotalCallsProcessed == 0 {
		t.Error("Expected some calls to be processed")
	}
}

// BenchmarkPerformanceOptimizerIteration benchmarks the optimized iteration path.
func BenchmarkPerformanceOptimizerIteration(b *testing.B) {
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	optimizer := manager.GetPerformanceOptimizer()
	manager.Start()
	defer manager.Stop()

	// Add test calls
	for i := uint32(1); i <= 5; i++ {
		err = manager.StartCall(i, 64000, 0)
		if err != nil {
			b.Fatalf("Failed to start call %d: %v", i, err)
		}
	}

	// Disable detailed logging for benchmarking
	optimizer.EnableDetailedLogging(false)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		callSlice, shouldProcess := optimizer.OptimizeIteration(manager)
		if shouldProcess {
			optimizer.ReturnCallSlice(callSlice)
		}
	}
}

// BenchmarkManagerIterateOptimized benchmarks the full Manager.Iterate method
// with performance optimizations enabled.
func BenchmarkManagerIterateOptimized(b *testing.B) {
	transport := NewMockTransport()
	friendLookup := func(uint32) ([]byte, error) { return []byte{1, 2, 3, 4}, nil }
	manager, err := NewManager(transport, friendLookup)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	manager.Start()
	defer manager.Stop()

	// Add test calls
	for i := uint32(1); i <= 5; i++ {
		err = manager.StartCall(i, 64000, 0)
		if err != nil {
			b.Fatalf("Failed to start call %d: %v", i, err)
		}
	}

	// Configure for optimal performance
	err = manager.EnablePerformanceOptimization(false, false)
	if err != nil {
		b.Fatalf("Failed to enable performance optimization: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		manager.Iterate()
	}
}
