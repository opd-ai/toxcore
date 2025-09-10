package main

import (
	"testing"
	"unsafe"
)

// TestToxAVCBindings tests the ToxAV C binding implementations
func TestToxAVCBindings(t *testing.T) {
	// Test toxav_new with nil pointer (should return nil)
	result := toxav_new(nil, nil)
	if result != nil {
		t.Error("Expected nil result when passing nil Tox instance")
	}

	// Test toxav_kill with nil pointer (should not crash)
	toxav_kill(nil)

	// Test toxav_iterate with nil pointer (should not crash)
	toxav_iterate(nil)

	// Test toxav_iteration_interval with nil pointer (should return default)
	interval := toxav_iteration_interval(nil)
	if interval != 20 {
		t.Errorf("Expected default interval of 20ms, got %d", interval)
	}

	// Test other functions with nil pointer (should return false)
	if toxav_call(nil, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call with nil")
	}

	if toxav_answer(nil, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_answer with nil")
	}

	if toxav_call_control(nil, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call_control with nil")
	}

	if toxav_audio_set_bit_rate(nil, 0, 0, nil) {
		t.Error("Expected false when calling toxav_audio_set_bit_rate with nil")
	}

	if toxav_video_set_bit_rate(nil, 0, 0, nil) {
		t.Error("Expected false when calling toxav_video_set_bit_rate with nil")
	}

	if toxav_audio_send_frame(nil, 0, nil, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_audio_send_frame with nil")
	}

	if toxav_video_send_frame(nil, 0, 0, 0, nil, nil, nil, nil) {
		t.Error("Expected false when calling toxav_video_send_frame with nil")
	}

	// Test callback registration functions with nil (should not crash)
	toxav_callback_call(nil, nil, nil)
	toxav_callback_call_state(nil, nil, nil)
	toxav_callback_audio_bit_rate(nil, nil, nil)
	toxav_callback_video_bit_rate(nil, nil, nil)
	toxav_callback_audio_receive_frame(nil, nil, nil)
	toxav_callback_video_receive_frame(nil, nil, nil)
}

// TestToxAVInstanceManagement tests the instance management functionality
func TestToxAVInstanceManagement(t *testing.T) {
	// Create a fake ToxAV ID for testing
	toxavMutex.Lock()
	fakeID := nextToxAVID
	nextToxAVID++
	toxavInstances[fakeID] = nil // Store nil instance for testing
	toxavMutex.Unlock()

	fakePtr := unsafe.Pointer(fakeID)

	// Test iteration interval with fake instance
	interval := toxav_iteration_interval(fakePtr)
	if interval != 20 {
		t.Errorf("Expected default interval of 20ms for nil instance, got %d", interval)
	}

	// Test iterate with fake instance (should not crash)
	toxav_iterate(fakePtr)

	// Test that calling functions with fake instance returns false
	if toxav_call(fakePtr, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call with nil instance")
	}

	// Test toxav_kill with fake instance
	toxav_kill(fakePtr)

	// Verify instance was removed
	toxavMutex.RLock()
	_, exists := toxavInstances[fakeID]
	toxavMutex.RUnlock()

	if exists {
		t.Error("Expected instance to be removed after toxav_kill")
	}
}

// TestThreadSafety tests basic thread safety of the instance management
func TestThreadSafety(t *testing.T) {
	// Create multiple fake instances concurrently
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Create fake instance
			toxavMutex.Lock()
			fakeID := nextToxAVID
			nextToxAVID++
			toxavInstances[fakeID] = nil
			toxavMutex.Unlock()

			fakePtr := unsafe.Pointer(fakeID)

			// Test operations
			toxav_iteration_interval(fakePtr)
			toxav_iterate(fakePtr)
			toxav_call(fakePtr, 0, 0, 0, nil)

			// Clean up
			toxav_kill(fakePtr)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkToxAVInstanceLookup benchmarks the instance lookup performance
func BenchmarkToxAVInstanceLookup(b *testing.B) {
	// Create a fake instance
	toxavMutex.Lock()
	fakeID := nextToxAVID
	nextToxAVID++
	toxavInstances[fakeID] = nil
	toxavMutex.Unlock()

	fakePtr := unsafe.Pointer(fakeID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toxav_iteration_interval(fakePtr)
	}

	// Clean up
	toxav_kill(fakePtr)
}

// TestErrorHandling tests error handling in C binding functions
func TestErrorHandling(t *testing.T) {
	// Test with invalid pointer values
	invalidPtr := unsafe.Pointer(uintptr(999999))

	// These should all handle invalid pointers gracefully
	if toxav_call(invalidPtr, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call with invalid pointer")
	}

	interval := toxav_iteration_interval(invalidPtr)
	if interval != 20 {
		t.Errorf("Expected default interval for invalid pointer, got %d", interval)
	}

	toxav_iterate(invalidPtr) // Should not crash
	toxav_kill(invalidPtr)    // Should not crash
}
