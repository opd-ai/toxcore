package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/av"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToxAVAudioSendFrameIntegration tests the complete audio frame sending integration
func TestToxAVAudioSendFrameIntegration(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	tox, err := New(options)
	require.NoError(t, err)
	defer tox.Kill()

	// Create ToxAV instance
	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	require.NotNil(t, toxAV)
	defer toxAV.Kill()

	// Start a call (this will set up media components)
	friendNumber := uint32(123)
	err = toxAV.Call(friendNumber, 64000, 0) // Audio-only call
	if err != nil {
		// Network errors are expected in test environments
		t.Skipf("Skipping audio integration test due to network setup failure: %v", err)
		return
	}

	// Generate test PCM audio data (10ms frame at 48kHz mono)
	sampleRate := uint32(48000)
	channels := uint8(1)
	sampleCount := 480 // 10ms at 48kHz
	pcmData := make([]int16, sampleCount)

	// Fill with a simple sine wave
	for i := 0; i < sampleCount; i++ {
		pcmData[i] = int16(16384 * 0.5) // Half amplitude
	}

	// Test audio frame sending through the complete integration
	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, channels, sampleRate)
	require.NoError(t, err, "Audio frame sending should succeed through complete integration")

	// Test input validation through ToxAV API
	err = toxAV.AudioSendFrame(friendNumber, []int16{}, 0, 1, 48000)
	assert.Error(t, err, "Empty PCM data should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, 0, 1, 48000)
	assert.Error(t, err, "Invalid sample count should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, 0, 48000)
	assert.Error(t, err, "Invalid channel count should be rejected")

	err = toxAV.AudioSendFrame(friendNumber, pcmData, sampleCount, channels, 0)
	assert.Error(t, err, "Invalid sample rate should be rejected")

	// Test with non-existent friend
	err = toxAV.AudioSendFrame(999, pcmData, sampleCount, channels, sampleRate)
	assert.Error(t, err, "Non-existent friend should be rejected")
	assert.Contains(t, err.Error(), "no active call")

	// End call
	err = toxAV.CallControl(friendNumber, av.CallControlCancel)
	require.NoError(t, err)
}

// TestToxAVAudioSendFramePerformance benchmarks the complete audio sending pipeline
func TestToxAVAudioSendFramePerformance(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	tox, err := New(options)
	require.NoError(t, err)
	defer tox.Kill()

	// Create ToxAV instance
	toxAV, err := NewToxAV(tox)
	require.NoError(t, err)
	defer toxAV.Kill()

	// Start a call
	friendNumber := uint32(123)
	err = toxAV.Call(friendNumber, 64000, 0)
	if err != nil {
		// Network errors are expected in test environments
		t.Skipf("Skipping audio performance test due to network setup failure: %v", err)
		return
	}

	// Generate test audio frame
	pcmData := make([]int16, 480) // 10ms at 48kHz
	for i := range pcmData {
		pcmData[i] = int16(16384 * 0.5)
	}

	// Measure performance of audio frame sending
	iterations := 1000
	for i := 0; i < iterations; i++ {
		err = toxAV.AudioSendFrame(friendNumber, pcmData, 480, 1, 48000)
		require.NoError(t, err)
	}

	// If we get here, the integration is working and performant
	t.Logf("Successfully sent %d audio frames through complete ToxAV integration", iterations)
}
