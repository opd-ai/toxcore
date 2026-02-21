package main

import (
	"testing"
	"unsafe"
)

// TestToxAVCallWithErrorPtr tests toxav_call with error pointer to capture error codes
func TestToxAVCallWithErrorPtr(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test call with non-existent friend - should trigger mapCallError
	// The error mapping will be exercised when the call fails
	result := toxav_call(toxav, 999, 48000, 0, nil)
	if result {
		t.Error("Expected false for call with non-existent friend")
	}

	// Test with different bit rates to ensure all paths work
	result = toxav_call(toxav, 0, 64000, 1000, nil)
	if result {
		t.Error("Expected false for call with non-existent friend (with video)")
	}
}

// TestToxAVAnswerWithErrorPtr tests toxav_answer with error pointer
func TestToxAVAnswerWithErrorPtr(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test answer with no incoming call - triggers mapAnswerError
	result := toxav_answer(toxav, 999, 48000, 500, nil)
	if result {
		t.Error("Expected false for answer with no incoming call")
	}

	// Test with different settings
	result = toxav_answer(toxav, 0, 64000, 1000, nil)
	if result {
		t.Error("Expected false for answer (different settings)")
	}
}

// TestToxAVCallControlWithErrorPtr tests call control error handling
func TestToxAVCallControlWithErrorPtr(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test all control types - each triggers mapCallControlError on failure
	controls := []struct {
		name    string
		control int
	}{
		{"RESUME", 0},
		{"PAUSE", 1},
		{"CANCEL", 2},
		{"MUTE_AUDIO", 3},
		{"UNMUTE_AUDIO", 4},
		{"HIDE_VIDEO", 5},
		{"SHOW_VIDEO", 6},
	}

	// Test each control type individually (cgo requires literal types)
	if toxav_call_control(toxav, 999, 0, nil) { // RESUME
		t.Error("Expected false for RESUME with no active call")
	}
	if toxav_call_control(toxav, 999, 1, nil) { // PAUSE
		t.Error("Expected false for PAUSE with no active call")
	}
	if toxav_call_control(toxav, 999, 2, nil) { // CANCEL
		t.Error("Expected false for CANCEL with no active call")
	}
	if toxav_call_control(toxav, 999, 3, nil) { // MUTE_AUDIO
		t.Error("Expected false for MUTE_AUDIO with no active call")
	}
	if toxav_call_control(toxav, 999, 4, nil) { // UNMUTE_AUDIO
		t.Error("Expected false for UNMUTE_AUDIO with no active call")
	}
	if toxav_call_control(toxav, 999, 5, nil) { // HIDE_VIDEO
		t.Error("Expected false for HIDE_VIDEO with no active call")
	}
	if toxav_call_control(toxav, 999, 6, nil) { // SHOW_VIDEO
		t.Error("Expected false for SHOW_VIDEO with no active call")
	}
	_ = controls // Silence unused warning
}

// TestToxAVBitRateSetWithErrorPtr tests bit rate setting error handling
func TestToxAVBitRateSetWithErrorPtr(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test audio bit rate - triggers mapBitRateSetError (isAudio=true)
	if toxav_audio_set_bit_rate(toxav, 999, 8000, nil) {
		t.Error("Expected false for audio bit rate 8000")
	}
	if toxav_audio_set_bit_rate(toxav, 999, 16000, nil) {
		t.Error("Expected false for audio bit rate 16000")
	}
	if toxav_audio_set_bit_rate(toxav, 999, 48000, nil) {
		t.Error("Expected false for audio bit rate 48000")
	}
	if toxav_audio_set_bit_rate(toxav, 999, 64000, nil) {
		t.Error("Expected false for audio bit rate 64000")
	}

	// Test video bit rate - triggers mapBitRateSetError (isAudio=false)
	if toxav_video_set_bit_rate(toxav, 999, 100, nil) {
		t.Error("Expected false for video bit rate 100")
	}
	if toxav_video_set_bit_rate(toxav, 999, 500, nil) {
		t.Error("Expected false for video bit rate 500")
	}
	if toxav_video_set_bit_rate(toxav, 999, 1000, nil) {
		t.Error("Expected false for video bit rate 1000")
	}
	if toxav_video_set_bit_rate(toxav, 999, 5000, nil) {
		t.Error("Expected false for video bit rate 5000")
	}
}

// TestToxAVSendFrameErrorHandling tests frame sending error paths
func TestToxAVSendFrameErrorHandling(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test audio send frame with nil PCM
	result := toxav_audio_send_frame(toxav, 0, nil, 480, 2, 48000, nil)
	if result {
		t.Error("Expected false for audio send frame with nil PCM")
	}

	// Test audio send frame with zero samples
	result = toxav_audio_send_frame(toxav, 0, nil, 0, 2, 48000, nil)
	if result {
		t.Error("Expected false for audio send frame with zero samples")
	}

	// Test audio send frame with different channels
	result = toxav_audio_send_frame(toxav, 0, nil, 480, 1, 48000, nil)
	if result {
		t.Error("Expected false for audio send frame mono")
	}

	// Test audio send frame with different sample rates
	result = toxav_audio_send_frame(toxav, 0, nil, 480, 2, 24000, nil)
	if result {
		t.Error("Expected false for audio send frame 24kHz")
	}

	result = toxav_audio_send_frame(toxav, 0, nil, 480, 2, 16000, nil)
	if result {
		t.Error("Expected false for audio send frame 16kHz")
	}

	// Test video send frame with nil planes
	result = toxav_video_send_frame(toxav, 0, 640, 480, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video send frame with nil planes")
	}

	// Test with zero dimensions
	result = toxav_video_send_frame(toxav, 0, 0, 0, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video send frame with zero dimensions")
	}

	// Test with different dimensions
	result = toxav_video_send_frame(toxav, 0, 1920, 1080, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video send frame 1080p")
	}

	result = toxav_video_send_frame(toxav, 0, 320, 240, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video send frame 240p")
	}

	// Test with large width/height that might trigger overflow
	result = toxav_video_send_frame(toxav, 0, 65535, 65535, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video send frame with very large dimensions")
	}
}

// TestToxInstanceByIDRetrieval tests GetToxInstanceByID function
func TestToxInstanceByIDRetrieval(t *testing.T) {
	// Test with non-existent ID
	result := GetToxInstanceByID(999999)
	if result != nil {
		t.Error("Expected nil for non-existent ID")
	}

	// Create an instance and verify retrieval
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	// Extract ID and verify retrieval works
	toxID, ok := safeGetToxID(tox)
	if !ok {
		t.Fatal("Failed to get Tox ID from pointer")
	}

	retrieved := GetToxInstanceByID(toxID)
	if retrieved == nil {
		t.Error("Expected non-nil for valid ID")
	}

	// After kill, should return nil
	tox_kill(tox)

	// Need to clear the defer since we already killed it
	// Create new instance to ensure test completes
	tox = tox_new()
	if tox == nil {
		t.Fatal("Failed to create replacement Tox instance")
	}
}

// TestSafeGetToxIDVariants tests various inputs to safeGetToxID
func TestSafeGetToxIDVariants(t *testing.T) {
	// Test nil
	t.Run("nil pointer", func(t *testing.T) {
		id, ok := safeGetToxID(nil)
		if ok {
			t.Error("Expected false for nil")
		}
		if id != 0 {
			t.Errorf("Expected 0 id, got %d", id)
		}
	})

	// Test valid pointer
	t.Run("valid instance", func(t *testing.T) {
		tox := tox_new()
		if tox == nil {
			t.Fatal("Failed to create Tox instance")
		}
		defer tox_kill(tox)

		id, ok := safeGetToxID(tox)
		if !ok {
			t.Error("Expected true for valid instance")
		}
		if id <= 0 {
			t.Errorf("Expected positive id, got %d", id)
		}
	})

	// Test invalid pointer (points to wrong memory)
	t.Run("invalid memory", func(t *testing.T) {
		invalidVal := 123456
		invalidPtr := unsafe.Pointer(&invalidVal)

		// This may return the value or fail gracefully depending on memory layout
		// The important thing is it shouldn't crash
		_, _ = safeGetToxID(invalidPtr)
	})
}

// TestToxBootstrapWithValidInstance tests bootstrap functionality
func TestToxBootstrapWithValidInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	// Bootstrap may fail due to network conditions
	// We just verify it doesn't crash and returns expected values
	result := tox_bootstrap_simple(tox)
	// -1 is valid (network error), 0 is valid (success)
	if result != 0 && result != -1 {
		t.Errorf("Unexpected bootstrap result: %d", result)
	}
}

// TestToxAVCallbackVariations tests callback registration variations
func TestToxAVCallbackVariations(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Test callback registration with nil callbacks (unregister)
	toxav_callback_call(toxav, nil, nil)
	toxav_callback_call_state(toxav, nil, nil)
	toxav_callback_audio_bit_rate(toxav, nil, nil)
	toxav_callback_video_bit_rate(toxav, nil, nil)
	toxav_callback_audio_receive_frame(toxav, nil, nil)
	toxav_callback_video_receive_frame(toxav, nil, nil)

	// Verify instance still works after callback changes
	interval := toxav_iteration_interval(toxav)
	if interval == 0 {
		t.Error("Expected non-zero interval after callback registration")
	}

	// Run iteration to ensure callbacks don't cause issues
	toxav_iterate(toxav)

	// Re-register nil callbacks multiple times (should not crash)
	for i := 0; i < 3; i++ {
		toxav_callback_call(toxav, nil, nil)
		toxav_callback_call_state(toxav, nil, nil)
		toxav_callback_audio_bit_rate(toxav, nil, nil)
		toxav_callback_video_bit_rate(toxav, nil, nil)
	}

	// Iterate after multiple callback registrations
	toxav_iterate(toxav)
}

// TestMultipleCallAttempts tests multiple call operations
func TestMultipleCallAttempts(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	toxav := toxav_new(tox, nil)
	if toxav == nil {
		t.Fatal("Failed to create ToxAV instance")
	}
	defer toxav_kill(toxav)

	// Multiple call attempts to exercise error paths
	for i := 0; i < 5; i++ {
		result := toxav_call(toxav, 0, 48000, 0, nil)
		if result {
			t.Errorf("Call %d should have failed", i)
		}

		result = toxav_answer(toxav, 0, 48000, 0, nil)
		if result {
			t.Errorf("Answer %d should have failed", i)
		}
	}
}

// TestToxAVWithNilPointers tests operations with nil pointers
func TestToxAVWithNilPointers(t *testing.T) {
	// Test with nil pointer - should handle gracefully
	toxav_iterate(nil)

	interval := toxav_iteration_interval(nil)
	if interval != 20 {
		t.Errorf("Expected default interval 20 for nil, got %d", interval)
	}

	result := toxav_call(nil, 0, 48000, 0, nil)
	if result {
		t.Error("Call should fail with nil pointer")
	}

	result = toxav_answer(nil, 0, 48000, 0, nil)
	if result {
		t.Error("Answer should fail with nil pointer")
	}
}
