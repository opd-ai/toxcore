package main

import (
	"testing"
	"unsafe"
)

// TestToxAVCallControl tests the toxav_call_control function
func TestToxAVCallControl(t *testing.T) {
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

	// Test call control with control type 0 (RESUME) - should fail since no call exists
	result := toxav_call_control(toxav, 0, 0, nil)
	if result {
		t.Error("Expected false for call_control with no active call")
	}

	// Test call control with control type 2 (CANCEL)
	result = toxav_call_control(toxav, 0, 2, nil)
	if result {
		t.Error("Expected false for call_control CANCEL with no active call")
	}
}

// TestToxAVAudioSetBitRate tests the toxav_audio_set_bit_rate function
func TestToxAVAudioSetBitRate(t *testing.T) {
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

	// Test setting audio bit rate (should fail - no active call)
	result := toxav_audio_set_bit_rate(toxav, 0, 48000, nil)
	if result {
		t.Error("Expected false for audio_set_bit_rate with no active call")
	}
}

// TestToxAVVideoSetBitRate tests the toxav_video_set_bit_rate function
func TestToxAVVideoSetBitRate(t *testing.T) {
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

	// Test setting video bit rate (should fail - no active call)
	result := toxav_video_set_bit_rate(toxav, 0, 1000, nil)
	if result {
		t.Error("Expected false for video_set_bit_rate with no active call")
	}
}

// TestToxAVAudioSendFrame tests the toxav_audio_send_frame function
func TestToxAVAudioSendFrame(t *testing.T) {
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

	// Test with nil PCM data
	result := toxav_audio_send_frame(toxav, 0, nil, 480, 2, 48000, nil)
	if result {
		t.Error("Expected false when sending audio with nil PCM")
	}
}

// TestToxAVVideoSendFrame tests the toxav_video_send_frame function
func TestToxAVVideoSendFrame(t *testing.T) {
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

	// Test with nil Y/U/V planes
	result := toxav_video_send_frame(toxav, 0, 640, 480, nil, nil, nil, nil)
	if result {
		t.Error("Expected false when sending video with nil planes")
	}
}

// TestToxAVFrameSendingNilAV tests frame sending with nil AV instance
func TestToxAVFrameSendingNilAV(t *testing.T) {
	result := toxav_audio_send_frame(nil, 0, nil, 480, 2, 48000, nil)
	if result {
		t.Error("Expected false for audio_send_frame with nil av")
	}

	result = toxav_video_send_frame(nil, 0, 640, 480, nil, nil, nil, nil)
	if result {
		t.Error("Expected false for video_send_frame with nil av")
	}
}

// TestToxAVCallWithBitRates tests call initiation with various bit rates
func TestToxAVCallWithBitRates(t *testing.T) {
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

	// Test calling with audio/video disabled
	result := toxav_call(toxav, 999, 0, 0, nil)
	if result {
		t.Error("Expected false for call with non-existent friend")
	}

	// Test with audio only
	result = toxav_call(toxav, 999, 48000, 0, nil)
	if result {
		t.Error("Expected false for audio-only call with non-existent friend")
	}
}

// TestToxAVAnswerWithBitRates tests call answering with various bit rates
func TestToxAVAnswerWithBitRates(t *testing.T) {
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

	// Test answering with no incoming call
	result := toxav_answer(toxav, 999, 48000, 500, nil)
	if result {
		t.Error("Expected false for answer with no incoming call")
	}
}

// TestToxAVInvalidInstanceID tests behavior with invalid ToxAV instance IDs
func TestToxAVInvalidInstanceID(t *testing.T) {
	// Create an invalid pointer
	invalidID := uintptr(999999)
	invalidPtr := unsafe.Pointer(&invalidID)

	// All operations should handle invalid IDs gracefully
	toxav_iterate(invalidPtr)

	interval := toxav_iteration_interval(invalidPtr)
	if interval != 20 { // Default value
		t.Errorf("Expected default interval for invalid ID, got %d", interval)
	}

	toxPtr := toxav_get_tox_from_av(invalidPtr)
	if toxPtr != nil {
		t.Error("Expected nil for get_tox_from_av with invalid ID")
	}

	result := toxav_call(invalidPtr, 0, 48000, 0, nil)
	if result {
		t.Error("Expected false for call with invalid ID")
	}

	result = toxav_answer(invalidPtr, 0, 48000, 0, nil)
	if result {
		t.Error("Expected false for answer with invalid ID")
	}

	result = toxav_call_control(invalidPtr, 0, 2, nil) // 2 = CANCEL
	if result {
		t.Error("Expected false for call_control with invalid ID")
	}

	result = toxav_audio_set_bit_rate(invalidPtr, 0, 48000, nil)
	if result {
		t.Error("Expected false for audio_set_bit_rate with invalid ID")
	}

	result = toxav_video_set_bit_rate(invalidPtr, 0, 1000, nil)
	if result {
		t.Error("Expected false for video_set_bit_rate with invalid ID")
	}
}

// TestToxAVCallbackRegistrationWithValidInstance tests callback registration on valid instance
func TestToxAVCallbackRegistrationWithValidInstance(t *testing.T) {
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

	// Test registering all callback types with nil callbacks
	// This should not crash and should update internal storage
	toxav_callback_call(toxav, nil, nil)
	toxav_callback_call_state(toxav, nil, nil)
	toxav_callback_audio_bit_rate(toxav, nil, nil)
	toxav_callback_video_bit_rate(toxav, nil, nil)
	toxav_callback_audio_receive_frame(toxav, nil, nil)
	toxav_callback_video_receive_frame(toxav, nil, nil)

	// Verify the instance is still functional after callback registration
	interval := toxav_iteration_interval(toxav)
	if interval == 0 {
		t.Error("Expected non-zero interval after callback registration")
	}

	// Iterate should still work
	toxav_iterate(toxav)
}

// TestToxAVCallbackRegistrationInvalidInstance tests callback registration with invalid instance
func TestToxAVCallbackRegistrationInvalidInstance(t *testing.T) {
	invalidID := uintptr(888888)
	invalidPtr := unsafe.Pointer(&invalidID)

	// These should not crash even with invalid instance
	toxav_callback_call(invalidPtr, nil, nil)
	toxav_callback_call_state(invalidPtr, nil, nil)
	toxav_callback_audio_bit_rate(invalidPtr, nil, nil)
	toxav_callback_video_bit_rate(invalidPtr, nil, nil)
	toxav_callback_audio_receive_frame(invalidPtr, nil, nil)
	toxav_callback_video_receive_frame(invalidPtr, nil, nil)
}

