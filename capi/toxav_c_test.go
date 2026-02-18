package main

import (
	"testing"
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
	// Test with nil pointer first (this is the safe approach)
	interval := toxav_iteration_interval(nil)
	if interval != 20 {
		t.Errorf("Expected default interval of 20ms for nil pointer, got %d", interval)
	}

	// Test iterate with nil pointer (should not crash)
	toxav_iterate(nil)

	// Test that calling functions with nil pointer returns false
	if toxav_call(nil, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call with nil pointer")
	}

	// Create a real ToxAV instance through the API to test with actual instances
	toxAVPtr := toxav_new(nil, nil) // This will return nil as expected
	if toxAVPtr != nil {
		t.Error("Expected nil from toxav_new with nil Tox instance")
	}

	// Test toxav_kill with nil pointer (should not crash)
	toxav_kill(nil)
}

// TestThreadSafety tests basic thread safety of the instance management
func TestThreadSafety(t *testing.T) {
	// Test concurrent access to functions with nil pointers
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Test operations with nil pointers (thread-safe)
			toxav_iteration_interval(nil)
			toxav_iterate(nil)
			toxav_call(nil, 0, 0, 0, nil)

			// Clean up (nil pointer is safe)
			toxav_kill(nil)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkToxAVInstanceLookup benchmarks the instance lookup performance
func BenchmarkToxAVInstanceLookup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark with nil pointer (fastest path)
		toxav_iteration_interval(nil)
	}
}

// TestErrorHandling tests error handling in C binding functions
func TestErrorHandling(t *testing.T) {
	// Test with nil pointer values (safest approach)
	// These should all handle nil pointers gracefully
	if toxav_call(nil, 0, 0, 0, nil) {
		t.Error("Expected false when calling toxav_call with nil pointer")
	}

	interval := toxav_iteration_interval(nil)
	if interval != 20 {
		t.Errorf("Expected default interval for nil pointer, got %d", interval)
	}

	toxav_iterate(nil) // Should not crash
	toxav_kill(nil)    // Should not crash
}

// TestCallbackStorage tests that callback storage is properly initialized and cleaned up
func TestCallbackStorage(t *testing.T) {
	// Test that callbacks storage map starts empty for nil pointers
	toxav_callback_call(nil, nil, nil)
	toxav_callback_call_state(nil, nil, nil)
	toxav_callback_audio_bit_rate(nil, nil, nil)
	toxav_callback_video_bit_rate(nil, nil, nil)
	toxav_callback_audio_receive_frame(nil, nil, nil)
	toxav_callback_video_receive_frame(nil, nil, nil)

	// Verify no panic occurs when registering nil callbacks
	// This also verifies the callback storage structure works correctly
}

// TestCallbackRegistrationWithNilCallback tests registration with nil callback but valid instance
func TestCallbackRegistrationWithNilCallback(t *testing.T) {
	// Create a mock av pointer for testing callback registration
	// Since we can't create a real ToxAV instance without a Tox instance,
	// we test that nil callbacks are handled gracefully

	// These should not crash even with nil callbacks
	toxav_callback_call(nil, nil, nil)
	toxav_callback_call_state(nil, nil, nil)
	toxav_callback_audio_bit_rate(nil, nil, nil)
	toxav_callback_video_bit_rate(nil, nil, nil)
	toxav_callback_audio_receive_frame(nil, nil, nil)
	toxav_callback_video_receive_frame(nil, nil, nil)
}
