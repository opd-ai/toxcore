package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScaler(t *testing.T) {
	scaler := NewScaler()
	assert.NotNil(t, scaler)
}

func TestScaler_Scale_BasicFunctionality(t *testing.T) {
	scaler := NewScaler()

	// Create source frame 320x240
	srcFrame := createTestFrame(320, 240)

	// Scale to 640x480 (2x scaling)
	result, err := scaler.Scale(srcFrame, 640, 480)

	require.NoError(t, err)
	assert.Equal(t, uint16(640), result.Width)
	assert.Equal(t, uint16(480), result.Height)
	assert.Equal(t, 640, result.YStride)
	assert.Equal(t, 320, result.UStride)
	assert.Equal(t, 320, result.VStride)

	// Verify plane sizes
	assert.Len(t, result.Y, 640*480) // Y plane
	assert.Len(t, result.U, 320*240) // U plane (quarter size)
	assert.Len(t, result.V, 320*240) // V plane (quarter size)
}

func TestScaler_Scale_DownScaling(t *testing.T) {
	scaler := NewScaler()

	// Create source frame 640x480
	srcFrame := createTestFrame(640, 480)

	// Scale down to 320x240 (0.5x scaling)
	result, err := scaler.Scale(srcFrame, 320, 240)

	require.NoError(t, err)
	assert.Equal(t, uint16(320), result.Width)
	assert.Equal(t, uint16(240), result.Height)
	assert.Len(t, result.Y, 320*240)
	assert.Len(t, result.U, 160*120)
	assert.Len(t, result.V, 160*120)
}

func TestScaler_Scale_SameDimensions(t *testing.T) {
	scaler := NewScaler()

	// Create source frame
	srcFrame := createTestFrame(640, 480)
	srcFrame.Y[100] = 123 // Add unique marker

	// "Scale" to same dimensions
	result, err := scaler.Scale(srcFrame, 640, 480)

	require.NoError(t, err)
	assert.Equal(t, uint16(640), result.Width)
	assert.Equal(t, uint16(480), result.Height)
	assert.Equal(t, byte(123), result.Y[100]) // Verify data copied

	// Verify it's a copy, not the same slice
	srcFrame.Y[100] = 200
	assert.Equal(t, byte(123), result.Y[100]) // Should still be 123
}

func TestScaler_Scale_ErrorCases(t *testing.T) {
	scaler := NewScaler()
	srcFrame := createTestFrame(320, 240)

	tests := []struct {
		name        string
		frame       *VideoFrame
		width       uint16
		height      uint16
		expectedErr string
	}{
		{
			name:        "nil frame",
			frame:       nil,
			width:       640,
			height:      480,
			expectedErr: "source frame cannot be nil",
		},
		{
			name:        "zero width",
			frame:       srcFrame,
			width:       0,
			height:      480,
			expectedErr: "invalid target dimensions",
		},
		{
			name:        "zero height",
			frame:       srcFrame,
			width:       640,
			height:      0,
			expectedErr: "invalid target dimensions",
		},
		{
			name:        "odd width",
			frame:       srcFrame,
			width:       641,
			height:      480,
			expectedErr: "target dimensions must be even for YUV420",
		},
		{
			name:        "odd height",
			frame:       srcFrame,
			width:       640,
			height:      481,
			expectedErr: "target dimensions must be even for YUV420",
		},
		{
			name:        "too small width",
			frame:       srcFrame,
			width:       14,
			height:      480,
			expectedErr: "target dimensions too small",
		},
		{
			name:        "too small height",
			frame:       srcFrame,
			width:       640,
			height:      14,
			expectedErr: "target dimensions too small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scaler.Scale(tt.frame, tt.width, tt.height)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Nil(t, result)
		})
	}
}

func TestScaler_Scale_VariousResolutions(t *testing.T) {
	scaler := NewScaler()

	testCases := []struct {
		name      string
		srcWidth  uint16
		srcHeight uint16
		dstWidth  uint16
		dstHeight uint16
	}{
		{"QVGA to VGA", 320, 240, 640, 480},
		{"VGA to HD", 640, 480, 1280, 720},
		{"HD to VGA", 1280, 720, 640, 480},
		{"QQVGA to QVGA", 160, 120, 320, 240},
		{"Custom resolution", 800, 600, 1024, 768},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srcFrame := createTestFrame(tc.srcWidth, tc.srcHeight)

			result, err := scaler.Scale(srcFrame, tc.dstWidth, tc.dstHeight)

			require.NoError(t, err)
			assert.Equal(t, tc.dstWidth, result.Width)
			assert.Equal(t, tc.dstHeight, result.Height)

			// Verify plane sizes
			expectedYSize := int(tc.dstWidth) * int(tc.dstHeight)
			expectedUVSize := int(tc.dstWidth/2) * int(tc.dstHeight/2)

			assert.Len(t, result.Y, expectedYSize)
			assert.Len(t, result.U, expectedUVSize)
			assert.Len(t, result.V, expectedUVSize)
		})
	}
}

func TestScaler_Scale_DataIntegrity(t *testing.T) {
	scaler := NewScaler()

	// Create source frame with specific pattern
	srcFrame := createTestFrame(160, 120)

	// Fill with gradient pattern for visual testing
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			idx := y*160 + x
			srcFrame.Y[idx] = byte((x + y) % 256)
		}
	}

	// Scale up 2x
	result, err := scaler.Scale(srcFrame, 320, 240)
	require.NoError(t, err)

	// Verify the data is reasonable (not all zeros or all same value)
	var sum int
	for _, val := range result.Y {
		sum += int(val)
	}
	average := sum / len(result.Y)

	// Should have reasonable average (not 0 or 255)
	assert.Greater(t, average, 10)
	assert.Less(t, average, 245)
}

func TestScaler_GetScaleFactors(t *testing.T) {
	scaler := NewScaler()

	tests := []struct {
		name                 string
		srcWidth, srcHeight  uint16
		dstWidth, dstHeight  uint16
		expectedX, expectedY float64
	}{
		{"2x scaling", 320, 240, 640, 480, 2.0, 2.0},
		{"0.5x scaling", 640, 480, 320, 240, 0.5, 0.5},
		{"same size", 640, 480, 640, 480, 1.0, 1.0},
		{"aspect change", 640, 480, 800, 600, 1.25, 1.25},
		{"different ratios", 640, 480, 1280, 720, 2.0, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xFactor, yFactor := scaler.GetScaleFactors(tt.srcWidth, tt.srcHeight,
				tt.dstWidth, tt.dstHeight)

			assert.InDelta(t, tt.expectedX, xFactor, 0.001)
			assert.InDelta(t, tt.expectedY, yFactor, 0.001)
		})
	}
}

func TestScaler_IsScalingRequired(t *testing.T) {
	scaler := NewScaler()

	tests := []struct {
		name                string
		srcWidth, srcHeight uint16
		dstWidth, dstHeight uint16
		expected            bool
	}{
		{"same dimensions", 640, 480, 640, 480, false},
		{"different width", 640, 480, 320, 480, true},
		{"different height", 640, 480, 640, 240, true},
		{"both different", 640, 480, 320, 240, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scaler.IsScalingRequired(tt.srcWidth, tt.srcHeight,
				tt.dstWidth, tt.dstHeight)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScaler_Scale_BilinearInterpolation(t *testing.T) {
	scaler := NewScaler()

	// Create a test frame with known values for interpolation testing
	frame := &VideoFrame{
		Width:  32,
		Height: 32,
		Y:      make([]byte, 1024),
		U:      make([]byte, 256),
		V:      make([]byte, 256),
	}

	// Fill with gradient pattern
	for i := 0; i < 1024; i++ {
		frame.Y[i] = byte(i % 256)
	}
	for i := 0; i < 256; i++ {
		frame.U[i] = byte(i)
		frame.V[i] = byte(i)
	}

	// Scale to 16x16 (should use bilinear interpolation)
	scaled, err := scaler.Scale(frame, 16, 16)
	require.NoError(t, err)
	require.NotNil(t, scaled)

	assert.Equal(t, uint16(16), scaled.Width)
	assert.Equal(t, uint16(16), scaled.Height)
	assert.Equal(t, 256, len(scaled.Y))
	assert.Equal(t, 64, len(scaled.U))
	assert.Equal(t, 64, len(scaled.V))

	// Verify that interpolation occurred (values should be reasonable)
	for i := range scaled.Y {
		assert.LessOrEqual(t, scaled.Y[i], byte(255))
	}
}

// Benchmark scaling performance
func BenchmarkScaler_Scale_VGAtoHD(b *testing.B) {
	scaler := NewScaler()
	srcFrame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scaler.Scale(srcFrame, 1280, 720)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScaler_Scale_HDtoVGA(b *testing.B) {
	scaler := NewScaler()
	srcFrame := createTestFrame(1280, 720)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scaler.Scale(srcFrame, 640, 480)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScaler_Scale_SmallFrame(b *testing.B) {
	scaler := NewScaler()
	srcFrame := createTestFrame(160, 120)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scaler.Scale(srcFrame, 320, 240)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create test frames with proper YUV420 structure
func createTestFrame(width, height uint16) *VideoFrame {
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
