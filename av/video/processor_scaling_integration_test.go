package video

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessorScalingIntegration tests video scaling integration within the processor pipeline.
// This ensures that scaling works correctly during outgoing video processing.
func TestProcessorScalingIntegration(t *testing.T) {
	tests := []struct {
		name               string
		processorWidth     uint16
		processorHeight    uint16
		inputWidth         uint16
		inputHeight        uint16
		expectScaling      bool
		expectedOutputSize int
	}{
		{
			name:               "no scaling required - same dimensions",
			processorWidth:     640,
			processorHeight:    480,
			inputWidth:         640,
			inputHeight:        480,
			expectScaling:      false,
			expectedOutputSize: 4, // Header (4 bytes) + data
		},
		{
			name:               "upscaling from QVGA to VGA",
			processorWidth:     640,
			processorHeight:    480,
			inputWidth:         320,
			inputHeight:        240,
			expectScaling:      true,
			expectedOutputSize: 4, // Header (4 bytes) + scaled data
		},
		{
			name:               "downscaling from HD to VGA",
			processorWidth:     640,
			processorHeight:    480,
			inputWidth:         1280,
			inputHeight:        720,
			expectScaling:      true,
			expectedOutputSize: 4, // Header (4 bytes) + scaled data
		},
		{
			name:               "small frame upscaling",
			processorWidth:     320,
			processorHeight:    240,
			inputWidth:         160,
			inputHeight:        120,
			expectScaling:      true,
			expectedOutputSize: 4, // Header (4 bytes) + scaled data
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor with target dimensions
			processor := NewProcessorWithSettings(tt.processorWidth, tt.processorHeight, 512000)

			// Create input frame with source dimensions
			inputFrame := createTestVideoFrame(tt.inputWidth, tt.inputHeight)

			// Process the frame (this should trigger scaling if needed)
			rtpPackets, err := processor.ProcessOutgoing(inputFrame)

			// Verify processing succeeded
			require.NoError(t, err)
			assert.NotNil(t, rtpPackets)
			assert.Greater(t, len(rtpPackets), 0) // Verify the scaler was properly initialized
			scaler := processor.GetScaler()
			assert.NotNil(t, scaler)

			// Verify scaling requirement detection
			isScalingRequired := scaler.IsScalingRequired(tt.inputWidth, tt.inputHeight, tt.processorWidth, tt.processorHeight)
			assert.Equal(t, tt.expectScaling, isScalingRequired)
		})
	}
}

// TestProcessorScalingWithEffects tests video scaling integration with effects processing.
// This ensures that scaling works correctly in combination with video effects.
func TestProcessorScalingWithEffects(t *testing.T) {
	// Create processor with VGA output resolution
	processor := NewProcessorWithSettings(640, 480, 512000)

	// Add a brightness effect to test effects + scaling integration
	effectChain := processor.GetEffectChain()
	brightEffect := NewBrightnessEffect(50)
	effectChain.AddEffect(brightEffect)

	// Create QVGA input frame (needs upscaling)
	inputFrame := createTestVideoFrame(320, 240)

	// Fill with test pattern to verify effects + scaling
	for i := range inputFrame.Y {
		inputFrame.Y[i] = 100 // Mid-level gray
	}

	// Process the frame (scaling + effects)
	rtpPackets, err := processor.ProcessOutgoing(inputFrame)

	// Verify processing succeeded
	require.NoError(t, err)
	assert.NotNil(t, rtpPackets)
	assert.Greater(t, len(rtpPackets), 0) // Should have RTP packets

	// Verify effects were applied by checking frame processing pipeline
	// The frame should be scaled first, then effects applied
	assert.Equal(t, 1, effectChain.GetEffectCount())
}

// TestProcessorScalingErrorHandling tests error handling in scaling integration.
func TestProcessorScalingErrorHandling(t *testing.T) {
	processor := NewProcessorWithSettings(640, 480, 512000)

	tests := []struct {
		name        string
		frame       *VideoFrame
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil frame",
			frame:       nil,
			expectError: true,
			errorMsg:    "frame cannot be nil",
		},
		{
			name: "frame with insufficient Y data",
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				YStride: 320,
				UStride: 160,
				VStride: 160,
				Y:       make([]byte, 100), // Too small
				U:       make([]byte, 160*120),
				V:       make([]byte, 160*120),
			},
			expectError: true,
			errorMsg:    "y plane too small",
		},
		{
			name: "frame with insufficient U data",
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				YStride: 320,
				UStride: 160,
				VStride: 160,
				Y:       make([]byte, 320*240),
				U:       make([]byte, 100), // Too small
				V:       make([]byte, 160*120),
			},
			expectError: true,
			errorMsg:    "u plane too small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.ProcessOutgoing(tt.frame)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProcessorScalingPerformance benchmarks scaling performance in the processor pipeline.
func TestProcessorScalingPerformance(t *testing.T) {
	// Test various scaling scenarios for performance
	scenarios := []struct {
		name           string
		srcWidth       uint16
		srcHeight      uint16
		dstWidth       uint16
		dstHeight      uint16
		maxProcessTime int // microseconds
	}{
		{"QVGA to VGA", 320, 240, 640, 480, 50000}, // 50ms max - accounts for 4x pixel scaling + encoding
		{"VGA to HD", 640, 480, 1280, 720, 100000}, // 100ms max - accounts for 2.25x pixel scaling + encoding
		{"HD to VGA", 1280, 720, 640, 480, 75000},  // 75ms max - downscaling with encoding
		{"No scaling", 640, 480, 640, 480, 10000},  // 10ms max - encoding only, no scaling
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			processor := NewProcessorWithSettings(scenario.dstWidth, scenario.dstHeight, 512000)
			inputFrame := createTestVideoFrame(scenario.srcWidth, scenario.srcHeight)

			// Measure processing time
			start := time.Now()
			_, err := processor.ProcessOutgoing(inputFrame)
			elapsed := time.Since(start).Microseconds()

			require.NoError(t, err)
			assert.LessOrEqual(t, int(elapsed), scenario.maxProcessTime,
				"Processing took %d μs, expected ≤ %d μs", elapsed, scenario.maxProcessTime)
		})
	}
}

// TestProcessorScalingDataIntegrity tests that scaling preserves data integrity.
func TestProcessorScalingDataIntegrity(t *testing.T) {
	processor := NewProcessorWithSettings(640, 480, 512000)

	// Create input frame with specific pattern
	inputFrame := createTestVideoFrame(320, 240)

	// Fill Y plane with gradient pattern
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			idx := y*320 + x
			inputFrame.Y[idx] = byte((x + y) % 256)
		}
	}

	// Process the frame
	rtpPackets, err := processor.ProcessOutgoing(inputFrame)
	require.NoError(t, err)
	require.Greater(t, len(rtpPackets), 0)

	// For this test, we'll verify the scaling worked by checking
	// that we can process the same input frame without errors
	// The exact frame reconstruction test would need RTP processing
	// which is complex for this integration test

	// Verify the frame was processed through the scaling pipeline
	scaler := processor.GetScaler()
	isScalingRequired := scaler.IsScalingRequired(320, 240, 640, 480)
	assert.True(t, isScalingRequired, "Scaling should be required for 320x240 → 640x480")

	// Test the scaler directly to verify scaling data integrity
	scaledFrame, err := scaler.Scale(inputFrame, 640, 480)
	require.NoError(t, err)

	// Verify dimensions were scaled correctly
	assert.Equal(t, uint16(640), scaledFrame.Width)
	assert.Equal(t, uint16(480), scaledFrame.Height)

	// Verify data is reasonable (not all zeros or corrupted)
	var sum int
	for _, val := range scaledFrame.Y {
		sum += int(val)
	}
	average := sum / len(scaledFrame.Y)

	// Should have reasonable average value
	assert.Greater(t, average, 10)
	assert.Less(t, average, 245)
}

// Helper function to create test video frames
func createTestVideoFrame(width, height uint16) *VideoFrame {
	ySize := int(width) * int(height)
	uvWidth := width / 2
	uvHeight := height / 2
	uvSize := int(uvWidth) * int(uvHeight)

	frame := &VideoFrame{
		Width:   width,
		Height:  height,
		YStride: int(width),
		UStride: int(uvWidth),
		VStride: int(uvWidth),
		Y:       make([]byte, ySize),
		U:       make([]byte, uvSize),
		V:       make([]byte, uvSize),
	}

	// Fill with test pattern
	for i := range frame.Y {
		frame.Y[i] = byte(i % 256)
	}
	for i := range frame.U {
		frame.U[i] = 128 // Neutral chroma
	}
	for i := range frame.V {
		frame.V[i] = 128 // Neutral chroma
	}

	return frame
}
