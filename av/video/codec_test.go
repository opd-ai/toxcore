package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVP8Codec(t *testing.T) {
	codec := NewVP8Codec()

	assert.NotNil(t, codec)
	assert.NotNil(t, codec.processor)
}

func TestVP8CodecEncodeFrame(t *testing.T) {
	codec := NewVP8Codec()

	tests := []struct {
		name      string
		frame     *VideoFrame
		expectErr bool
	}{
		{
			name: "valid_frame",
			frame: &VideoFrame{
				Width:   640,
				Height:  480,
				Y:       make([]byte, 640*480),
				U:       make([]byte, 640*480/4),
				V:       make([]byte, 640*480/4),
				YStride: 640,
				UStride: 320,
				VStride: 320,
			},
			expectErr: false,
		},
		{
			name:      "nil_frame",
			frame:     nil,
			expectErr: true,
		},
		{
			name: "invalid_dimensions",
			frame: &VideoFrame{
				Width:  0,
				Height: 480,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := codec.EncodeFrame(tt.frame)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

func TestVP8CodecDecodeFrame(t *testing.T) {
	codec := NewVP8Codec()

	// Create test frame with matching dimensions (640x480 - default processor size)
	testFrame := &VideoFrame{
		Width:   640,
		Height:  480,
		Y:       make([]byte, 640*480),
		U:       make([]byte, 640*480/4),
		V:       make([]byte, 640*480/4),
		YStride: 640,
		UStride: 320,
		VStride: 320,
	}

	// Fill with test pattern
	for i := range testFrame.Y {
		testFrame.Y[i] = byte(i % 256)
	}
	for i := range testFrame.U {
		testFrame.U[i] = byte((i + 128) % 256)
	}
	for i := range testFrame.V {
		testFrame.V[i] = byte((i + 64) % 256)
	}

	// Encode first
	data, err := codec.EncodeFrame(testFrame)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// Test decoding
	tests := []struct {
		name      string
		data      []byte
		expectErr bool
	}{
		{
			name:      "valid_data",
			data:      data,
			expectErr: false,
		},
		{
			name:      "invalid_data",
			data:      []byte{1, 2, 3},
			expectErr: true,
		},
		{
			name:      "empty_data",
			data:      []byte{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := codec.DecodeFrame(tt.data)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, frame)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, frame)

				// Verify decoded frame matches original
				assert.Equal(t, testFrame.Width, frame.Width)
				assert.Equal(t, testFrame.Height, frame.Height)
				assert.Equal(t, testFrame.Y, frame.Y)
				assert.Equal(t, testFrame.U, frame.U)
				assert.Equal(t, testFrame.V, frame.V)
			}
		})
	}
}

func TestVP8CodecSetBitRate(t *testing.T) {
	codec := NewVP8Codec()

	tests := []struct {
		name      string
		bitRate   uint32
		expectErr bool
	}{
		{
			name:      "valid_bitrate",
			bitRate:   1000000,
			expectErr: false,
		},
		{
			name:      "zero_bitrate",
			bitRate:   0,
			expectErr: true,
		},
		{
			name:      "high_bitrate",
			bitRate:   8000000,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := codec.SetBitRate(tt.bitRate)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVP8CodecGetSupportedResolutions(t *testing.T) {
	codec := NewVP8Codec()

	resolutions := codec.GetSupportedResolutions()
	assert.NotEmpty(t, resolutions)

	// Check some expected resolutions
	expectedResolutions := []Resolution{
		{Width: 160, Height: 120},   // QQVGA
		{Width: 320, Height: 240},   // QVGA
		{Width: 640, Height: 480},   // VGA
		{Width: 1280, Height: 720},  // HD 720p
		{Width: 1920, Height: 1080}, // HD 1080p
	}

	for _, expected := range expectedResolutions {
		found := false
		for _, resolution := range resolutions {
			if resolution.Width == expected.Width && resolution.Height == expected.Height {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected resolution %s not found", expected.String())
	}
}

func TestVP8CodecGetSupportedBitRates(t *testing.T) {
	codec := NewVP8Codec()

	bitRates := codec.GetSupportedBitRates()
	assert.NotEmpty(t, bitRates)

	// Check some expected bit rates
	expectedBitRates := []uint32{64000, 128000, 256000, 512000, 1000000, 2000000, 4000000, 8000000}

	for _, expected := range expectedBitRates {
		found := false
		for _, bitRate := range bitRates {
			if bitRate == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected bit rate %d not found", expected)
	}
}

func TestVP8CodecValidateFrameSize(t *testing.T) {
	codec := NewVP8Codec()

	tests := []struct {
		name      string
		width     uint16
		height    uint16
		expectErr bool
	}{
		{
			name:      "valid_size",
			width:     640,
			height:    480,
			expectErr: false,
		},
		{
			name:      "odd_width",
			width:     641,
			height:    480,
			expectErr: true,
		},
		{
			name:      "odd_height",
			width:     640,
			height:    481,
			expectErr: true,
		},
		{
			name:      "too_small",
			width:     8,
			height:    8,
			expectErr: true,
		},
		{
			name:      "too_large",
			width:     20000,
			height:    20000,
			expectErr: true,
		},
		{
			name:      "minimum_valid",
			width:     16,
			height:    16,
			expectErr: false,
		},
		{
			name:      "maximum_valid",
			width:     16382,
			height:    16382,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := codec.ValidateFrameSize(tt.width, tt.height)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVP8CodecClose(t *testing.T) {
	codec := NewVP8Codec()

	err := codec.Close()
	assert.NoError(t, err)
}

func TestResolutionString(t *testing.T) {
	resolution := Resolution{Width: 1280, Height: 720}
	assert.Equal(t, "1280x720", resolution.String())
}

func TestGetBitrateForResolution(t *testing.T) {
	tests := []struct {
		name       string
		resolution Resolution
		expected   uint32
	}{
		{
			name:       "QQVGA",
			resolution: Resolution{Width: 160, Height: 120},
			expected:   64000,
		},
		{
			name:       "QVGA",
			resolution: Resolution{Width: 320, Height: 240},
			expected:   128000,
		},
		{
			name:       "VGA",
			resolution: Resolution{Width: 640, Height: 480},
			expected:   512000,
		},
		{
			name:       "HD_720p",
			resolution: Resolution{Width: 1280, Height: 720},
			expected:   2000000,
		},
		{
			name:       "HD_1080p",
			resolution: Resolution{Width: 1920, Height: 1080},
			expected:   4000000,
		},
		{
			name:       "Very_large",
			resolution: Resolution{Width: 4096, Height: 2160},
			expected:   8000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitrate := GetBitrateForResolution(tt.resolution)
			assert.Equal(t, tt.expected, bitrate)
		})
	}
}

// Benchmark tests
func BenchmarkVP8CodecEncodeFrame(b *testing.B) {
	codec := NewVP8Codec()
	frame := &VideoFrame{
		Width:   640,
		Height:  480,
		Y:       make([]byte, 640*480),
		U:       make([]byte, 640*480/4),
		V:       make([]byte, 640*480/4),
		YStride: 640,
		UStride: 320,
		VStride: 320,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.EncodeFrame(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVP8CodecRoundTrip(b *testing.B) {
	codec := NewVP8Codec()
	frame := &VideoFrame{
		Width:   640,
		Height:  480,
		Y:       make([]byte, 640*480),
		U:       make([]byte, 640*480/4),
		V:       make([]byte, 640*480/4),
		YStride: 640,
		UStride: 320,
		VStride: 320,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := codec.EncodeFrame(frame)
		if err != nil {
			b.Fatal(err)
		}

		_, err = codec.DecodeFrame(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
