package toxcore

import (
	"testing"
	"time"

	avpkg "github.com/opd-ai/toxforge/av"
)

// TestNewToxAV verifies ToxAV creation and basic functionality.
func TestNewToxAV(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}

	if toxav == nil {
		t.Fatal("ToxAV should not be nil")
	}

	// Test IterationInterval
	interval := toxav.IterationInterval()
	expectedInterval := 20 * time.Millisecond
	if interval != expectedInterval {
		t.Errorf("Expected iteration interval %v, got %v", expectedInterval, interval)
	}

	// Test Iterate (should not panic)
	toxav.Iterate()

	// Clean up
	toxav.Kill()
}

// TestNewToxAVWithNilTox verifies error handling for nil Tox instance.
func TestNewToxAVWithNilTox(t *testing.T) {
	toxav, err := NewToxAV(nil)
	if err == nil {
		t.Error("Expected error when creating ToxAV with nil Tox instance")
	}
	if toxav != nil {
		t.Error("ToxAV should be nil when creation fails")
	}
}

// TestToxAVCallManagement verifies basic call management API.
func TestToxAVCallManagement(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(123)
	audioBitRate := uint32(64000)
	videoBitRate := uint32(1000000)

	// Test call initiation
	err = toxav.Call(friendNumber, audioBitRate, videoBitRate)
	if err != nil {
		t.Fatalf("Failed to initiate call: %v", err)
	}

	// Test call control
	err = toxav.CallControl(friendNumber, avpkg.CallControlCancel)
	if err != nil {
		t.Fatalf("Failed to control call: %v", err)
	}
}

// TestToxAVBitRateManagement verifies bit rate management API.
func TestToxAVBitRateManagement(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(456)

	// Start a call first
	err = toxav.Call(friendNumber, 64000, 1000000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Test audio bit rate setting
	err = toxav.AudioSetBitRate(friendNumber, 32000)
	if err != nil {
		t.Fatalf("Failed to set audio bit rate: %v", err)
	}

	// Test video bit rate setting
	err = toxav.VideoSetBitRate(friendNumber, 500000)
	if err != nil {
		t.Fatalf("Failed to set video bit rate: %v", err)
	}
}

// TestToxAVCallControl verifies call control functionality.
func TestToxAVCallControl(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(789)

	// Start a call
	err = toxav.Call(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Test various control commands
	controls := []avpkg.CallControl{
		avpkg.CallControlPause,  // Should return "not implemented" error
		avpkg.CallControlResume, // Should return "not implemented" error
		avpkg.CallControlCancel, // Should work
	}

	for _, control := range controls {
		err = toxav.CallControl(friendNumber, control)
		// We expect errors for unimplemented controls, but no panics
		if control == avpkg.CallControlCancel && err != nil {
			t.Errorf("CallControlCancel should work, got error: %v", err)
		}
	}
}

// TestToxAVFrameSending verifies frame sending API (currently unimplemented).
func TestToxAVFrameSending(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(321)

	// Test audio frame sending (should return "not implemented" error)
	pcm := make([]int16, 1920) // 48kHz * 0.02s * 2 channels
	err = toxav.AudioSendFrame(friendNumber, pcm, 960, 2, 48000)
	if err == nil {
		t.Error("Expected error for unimplemented audio frame sending")
	}

	// Test video frame sending (should return "not implemented" error)
	width, height := uint16(640), uint16(480)
	ySize := int(width) * int(height)
	uvSize := ySize / 4
	y := make([]byte, ySize)
	u := make([]byte, uvSize)
	v := make([]byte, uvSize)

	err = toxav.VideoSendFrame(friendNumber, width, height, y, u, v)
	if err == nil {
		t.Error("Expected error for unimplemented video frame sending")
	}
}

// TestToxAVCallbacks verifies callback registration.
func TestToxAVCallbacks(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	// Test callback registration (should not panic)
	toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		// Test callback
	})

	toxav.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
		// Test callback
	})

	toxav.CallbackAudioBitRate(func(friendNumber uint32, bitRate uint32) {
		// Test callback
	})

	toxav.CallbackVideoBitRate(func(friendNumber uint32, bitRate uint32) {
		// Test callback
	})

	toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		// Test callback
	})

	toxav.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		// Test callback
	})
}

// TestToxAVKill verifies proper cleanup.
func TestToxAVKill(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}

	// Start a call
	err = toxav.Call(123, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Kill the instance
	toxav.Kill()

	// Operations after Kill should fail gracefully
	err = toxav.Call(456, 64000, 0)
	if err == nil {
		t.Error("Expected error when calling after Kill()")
	}

	err = toxav.AudioSetBitRate(123, 32000)
	if err == nil {
		t.Error("Expected error when setting bit rate after Kill()")
	}

	// Iterate should not panic
	toxav.Iterate()

	// IterationInterval should still work
	interval := toxav.IterationInterval()
	if interval <= 0 {
		t.Error("IterationInterval should return positive value even after Kill()")
	}
}

// TestToxAVAnswerCall verifies call answering functionality.
func TestToxAVAnswerCall(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(654)

	// Test answering non-existent call
	err = toxav.Answer(friendNumber, 64000, 0)
	if err == nil {
		t.Error("Expected error when answering non-existent call")
	}

	// In a real implementation, we would simulate an incoming call
	// For now, just test that the Answer method exists and handles errors
}

// TestToxAVInvalidCallControl verifies handling of invalid call control values.
func TestToxAVInvalidCallControl(t *testing.T) {
	// Create a properly initialized Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill() // Ensure proper cleanup

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV: %v", err)
	}
	defer toxav.Kill()

	friendNumber := uint32(987)

	// Start a call
	err = toxav.Call(friendNumber, 64000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Test invalid call control value
	invalidControl := avpkg.CallControl(999)
	err = toxav.CallControl(friendNumber, invalidControl)
	if err == nil {
		t.Error("Expected error for invalid call control value")
	}
}
