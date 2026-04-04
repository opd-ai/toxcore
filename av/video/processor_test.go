package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProcessor(t *testing.T) {
	processor := NewProcessor()

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.encoder)
	assert.Equal(t, uint16(640), processor.width)
	assert.Equal(t, uint16(480), processor.height)
	assert.Equal(t, uint32(512000), processor.bitRate)

	// Verify it's a real VP8 encoder
	assert.IsType(t, &RealVP8Encoder{}, processor.encoder)
}

func TestNewProcessorWithSettings(t *testing.T) {
	width := uint16(1280)
	height := uint16(720)
	bitRate := uint32(2000000)

	processor := NewProcessorWithSettings(width, height, bitRate)

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.encoder)
	assert.Equal(t, width, processor.width)
	assert.Equal(t, height, processor.height)
	assert.Equal(t, bitRate, processor.bitRate)

	// Verify it's a real VP8 encoder
	assert.IsType(t, &RealVP8Encoder{}, processor.encoder)
}

func TestRealVP8Encoder(t *testing.T) {
	tests := []struct {
		name      string
		width     uint16
		height    uint16
		bitRate   uint32
		frame     *VideoFrame
		expectErr bool
	}{
		{
			name:    "valid_encoding_320x240",
			width:   320,
			height:  240,
			bitRate: 256000,
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				Y:       make([]byte, 320*240),
				U:       make([]byte, 320*240/4),
				V:       make([]byte, 320*240/4),
				YStride: 320,
				UStride: 160,
				VStride: 160,
			},
			expectErr: false,
		},
		{
			name:    "frame_size_mismatch",
			width:   640,
			height:  480,
			bitRate: 512000,
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				Y:       make([]byte, 320*240),
				U:       make([]byte, 320*240/4),
				V:       make([]byte, 320*240/4),
				YStride: 320,
				UStride: 160,
				VStride: 160,
			},
			expectErr: true,
		},
		{
			name:    "valid_encoding_640x480",
			width:   640,
			height:  480,
			bitRate: 512000,
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
			width:     320,
			height:    240,
			bitRate:   256000,
			frame:     nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder, err := NewRealVP8Encoder(tt.width, tt.height, tt.bitRate)
			assert.NoError(t, err)
			assert.NotNil(t, encoder)

			data, err := encoder.Encode(tt.frame)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				// VP8 encoded data should be non-trivial
				assert.Greater(t, len(data), 10, "VP8 encoded data should have reasonable size")
			}
		})
	}
}

func TestRealVP8EncoderStridedFrame(t *testing.T) {
	encoder, err := NewRealVP8Encoder(320, 240, 256000)
	assert.NoError(t, err)

	// Create a frame with stride larger than width (e.g., stride 384 for width 320)
	stride := 384
	uvStride := 192
	h := 240
	uvH := h / 2

	frame := &VideoFrame{
		Width:   320,
		Height:  240,
		Y:       make([]byte, stride*h),
		U:       make([]byte, uvStride*uvH),
		V:       make([]byte, uvStride*uvH),
		YStride: stride,
		UStride: uvStride,
		VStride: uvStride,
	}

	// Fill Y plane with a pattern (only active pixels matter)
	for y := 0; y < h; y++ {
		for x := 0; x < 320; x++ {
			frame.Y[y*stride+x] = byte((x + y) % 256)
		}
	}
	for y := 0; y < uvH; y++ {
		for x := 0; x < 160; x++ {
			frame.U[y*uvStride+x] = 128
			frame.V[y*uvStride+x] = 128
		}
	}

	data, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Greater(t, len(data), 10)
}

func TestRealVP8EncoderSetBitRate(t *testing.T) {
	encoder, err := NewRealVP8Encoder(640, 480, 512000)
	assert.NoError(t, err)

	err = encoder.SetBitRate(1000000)
	assert.NoError(t, err)
	assert.Equal(t, uint32(1000000), encoder.bitRate)
}

func TestRealVP8EncoderClose(t *testing.T) {
	encoder, err := NewRealVP8Encoder(640, 480, 512000)
	assert.NoError(t, err)

	err = encoder.Close()
	assert.NoError(t, err)
}

func TestRealVP8EncoderInterFrame(t *testing.T) {
	encoder, err := NewRealVP8Encoder(320, 240, 256000)
	assert.NoError(t, err)
	assert.True(t, encoder.SupportsInterframe())

	frame := &VideoFrame{
		Width:   320,
		Height:  240,
		Y:       make([]byte, 320*240),
		U:       make([]byte, 320*240/4),
		V:       make([]byte, 320*240/4),
		YStride: 320,
		UStride: 160,
		VStride: 160,
	}

	// Fill with a pattern
	for i := range frame.Y {
		frame.Y[i] = byte(i % 256)
	}
	for i := range frame.U {
		frame.U[i] = 128
	}
	for i := range frame.V {
		frame.V[i] = 128
	}

	// Encode first frame (should be a key frame)
	data1, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data1)
	assert.Greater(t, len(data1), 10)

	// Encode second frame (should be an inter frame with default interval=30)
	data2, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data2)
	assert.Greater(t, len(data2), 0)
}

func TestRealVP8EncoderSetKeyFrameInterval(t *testing.T) {
	encoder, err := NewRealVP8Encoder(320, 240, 256000)
	assert.NoError(t, err)

	// Set key frame interval to every frame (I-frame only mode)
	encoder.SetKeyFrameInterval(0)

	frame := &VideoFrame{
		Width:   320,
		Height:  240,
		Y:       make([]byte, 320*240),
		U:       make([]byte, 320*240/4),
		V:       make([]byte, 320*240/4),
		YStride: 320,
		UStride: 160,
		VStride: 160,
	}

	// Both frames should be key frames when interval is 0
	data1, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data1)

	data2, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data2)
}

func TestRealVP8EncoderForceKeyFrame(t *testing.T) {
	encoder, err := NewRealVP8Encoder(320, 240, 256000)
	assert.NoError(t, err)

	frame := &VideoFrame{
		Width:   320,
		Height:  240,
		Y:       make([]byte, 320*240),
		U:       make([]byte, 320*240/4),
		V:       make([]byte, 320*240/4),
		YStride: 320,
		UStride: 160,
		VStride: 160,
	}

	// Encode first frame (key frame)
	_, err = encoder.Encode(frame)
	assert.NoError(t, err)

	// Encode second frame (inter frame)
	_, err = encoder.Encode(frame)
	assert.NoError(t, err)

	// Force next frame to be a key frame
	encoder.ForceKeyFrame()

	// This frame should be a key frame
	data3, err := encoder.Encode(frame)
	assert.NoError(t, err)
	assert.NotNil(t, data3)
	assert.Greater(t, len(data3), 0)
}

func TestRealVP8EncoderMultiFrameRoundTrip(t *testing.T) {
	processor := NewProcessor()

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

	// Fill with uniform values for reliable key frame decoding
	for i := range frame.Y {
		frame.Y[i] = 128
	}
	for i := range frame.U {
		frame.U[i] = 128
	}
	for i := range frame.V {
		frame.V[i] = 128
	}

	// Encode multiple frames (mix of key and inter frames)
	for i := 0; i < 5; i++ {
		data, err := processor.ProcessOutgoingLegacy(frame)
		assert.NoError(t, err, "Frame %d encoding failed", i)
		assert.NotNil(t, data, "Frame %d produced nil data", i)
		assert.Greater(t, len(data), 0, "Frame %d produced empty data", i)

		// Decode each frame back (inter frames return cached key frame)
		decoded, err := processor.ProcessIncomingLegacy(data)
		assert.NoError(t, err, "Frame %d decoding failed", i)
		assert.NotNil(t, decoded, "Frame %d decoded to nil", i)
		assert.Equal(t, frame.Width, decoded.Width)
		assert.Equal(t, frame.Height, decoded.Height)
	}
}

func TestEncoderSupportsInterframe(t *testing.T) {
	// Test RealVP8Encoder (pure Go, with P-frame support)
	realEncoder, err := NewRealVP8Encoder(640, 480, 512000)
	assert.NoError(t, err)
	assert.True(t, realEncoder.SupportsInterframe(),
		"RealVP8Encoder should support inter-frame prediction (P-frames)")

	// Test SimpleVP8Encoder (passthrough)
	simpleEncoder := NewSimpleVP8Encoder(640, 480, 512000)
	assert.False(t, simpleEncoder.SupportsInterframe(),
		"SimpleVP8Encoder should not support inter-frame")

	// Test that they both implement Encoder interface
	var _ Encoder = realEncoder
	var _ Encoder = simpleEncoder
}

func TestDefaultEncoderFactory(t *testing.T) {
	// Test the default encoder factory
	encoder, err := NewDefaultEncoder(640, 480, 512000)
	assert.NoError(t, err)
	assert.NotNil(t, encoder)

	// In pure-Go builds, should support inter-frame via opd-ai/vp8
	assert.True(t, DefaultEncoderSupportsInterframe(),
		"Default encoder should support inter-frame via opd-ai/vp8")

	// Test encoder name
	name := DefaultEncoderName()
	assert.Contains(t, name, "vp8", "Encoder name should mention VP8")
	assert.Contains(t, name, "P-frame", "Encoder name should mention P-frame support")
}

func TestSimpleVP8Encoder(t *testing.T) {
	tests := []struct {
		name      string
		width     uint16
		height    uint16
		bitRate   uint32
		frame     *VideoFrame
		expectErr bool
	}{
		{
			name:    "valid_encoding",
			width:   320,
			height:  240,
			bitRate: 256000,
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				Y:       make([]byte, 320*240),
				U:       make([]byte, 320*240/4),
				V:       make([]byte, 320*240/4),
				YStride: 320,
				UStride: 160,
				VStride: 160,
			},
			expectErr: false,
		},
		{
			name:    "frame_size_mismatch",
			width:   640,
			height:  480,
			bitRate: 512000,
			frame: &VideoFrame{
				Width:   320,
				Height:  240,
				Y:       make([]byte, 320*240),
				U:       make([]byte, 320*240/4),
				V:       make([]byte, 320*240/4),
				YStride: 320,
				UStride: 160,
				VStride: 160,
			},
			expectErr: true,
		},
		{
			name:    "small_frame",
			width:   160,
			height:  120,
			bitRate: 64000,
			frame: &VideoFrame{
				Width:   160,
				Height:  120,
				Y:       make([]byte, 160*120),
				U:       make([]byte, 160*120/4),
				V:       make([]byte, 160*120/4),
				YStride: 160,
				UStride: 80,
				VStride: 80,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewSimpleVP8Encoder(tt.width, tt.height, tt.bitRate)
			assert.NotNil(t, encoder)

			data, err := encoder.Encode(tt.frame)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)

				// Verify the encoded data has the expected structure
				assert.GreaterOrEqual(t, len(data), 4) // At least header

				// Check dimensions in header (little-endian)
				width := uint16(data[0]) | (uint16(data[1]) << 8)
				height := uint16(data[2]) | (uint16(data[3]) << 8)
				assert.Equal(t, tt.frame.Width, width)
				assert.Equal(t, tt.frame.Height, height)
			}
		})
	}
}

func TestSimpleVP8EncoderSetBitRate(t *testing.T) {
	encoder := NewSimpleVP8Encoder(640, 480, 512000)

	err := encoder.SetBitRate(1000000)
	assert.NoError(t, err)
	assert.Equal(t, uint32(1000000), encoder.bitRate)
}

func TestSimpleVP8EncoderClose(t *testing.T) {
	encoder := NewSimpleVP8Encoder(640, 480, 512000)

	err := encoder.Close()
	assert.NoError(t, err)
}

func TestProcessorProcessOutgoing(t *testing.T) {
	processor := NewProcessor()

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
			name: "zero_dimensions",
			frame: &VideoFrame{
				Width:  0,
				Height: 480,
			},
			expectErr: true,
		},
		{
			name: "invalid_y_size",
			frame: &VideoFrame{
				Width:   640,
				Height:  480,
				Y:       make([]byte, 100), // Too small
				U:       make([]byte, 640*480/4),
				V:       make([]byte, 640*480/4),
				YStride: 640,
				UStride: 320,
				VStride: 320,
			},
			expectErr: true,
		},
		{
			name: "invalid_u_size",
			frame: &VideoFrame{
				Width:   640,
				Height:  480,
				Y:       make([]byte, 640*480),
				U:       make([]byte, 100), // Too small
				V:       make([]byte, 640*480/4),
				YStride: 640,
				UStride: 320,
				VStride: 320,
			},
			expectErr: true,
		},
		{
			name: "invalid_v_size",
			frame: &VideoFrame{
				Width:   640,
				Height:  480,
				Y:       make([]byte, 640*480),
				U:       make([]byte, 640*480/4),
				V:       make([]byte, 100), // Too small
				YStride: 640,
				UStride: 320,
				VStride: 320,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := processor.ProcessOutgoing(tt.frame)

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

func TestProcessorProcessIncoming(t *testing.T) {
	processor := NewProcessor()

	// Create test frame with matching dimensions (640x480 - default processor size)
	// Use uniform content which the encoder produces decodable key frames for.
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

	// Fill with uniform values
	for i := range testFrame.Y {
		testFrame.Y[i] = 128
	}
	for i := range testFrame.U {
		testFrame.U[i] = 128
	}
	for i := range testFrame.V {
		testFrame.V[i] = 128
	}

	// Encode it first using VP8
	data, err := processor.ProcessOutgoingLegacy(testFrame)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Greater(t, len(data), 10, "VP8 encoded data should have reasonable size")

	tests := []struct {
		name      string
		data      []byte
		expectErr bool
	}{
		{
			name:      "valid_vp8_data",
			data:      data,
			expectErr: false,
		},
		{
			name:      "too_short",
			data:      []byte{1, 2},
			expectErr: true,
		},
		{
			name:      "inter_frame_returns_cached",
			data:      []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // bit 0 = 1 → inter frame
			expectErr: false,                                       // returns cached key frame
		},
		{
			name:      "empty_data",
			data:      []byte{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := processor.ProcessIncomingLegacy(tt.data)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, frame)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, frame)

				// Verify frame dimensions match (VP8 preserves dimensions)
				assert.Equal(t, testFrame.Width, frame.Width)
				assert.Equal(t, testFrame.Height, frame.Height)

				// Verify plane sizes are correct for YUV420
				assert.Equal(t, int(frame.Width)*int(frame.Height), len(frame.Y))
				assert.Equal(t, int(frame.Width)*int(frame.Height)/4, len(frame.U))
				assert.Equal(t, int(frame.Width)*int(frame.Height)/4, len(frame.V))
			}
		})
	}
}

func TestProcessorSetBitRate(t *testing.T) {
	processor := NewProcessor()

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
			err := processor.SetBitRate(tt.bitRate)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.bitRate, processor.GetBitRate())
			}
		})
	}
}

func TestProcessorFrameSize(t *testing.T) {
	processor := NewProcessor()

	// Test getting initial frame size
	width, height := processor.GetFrameSize()
	assert.Equal(t, uint16(640), width)
	assert.Equal(t, uint16(480), height)

	// Test setting new frame size
	err := processor.SetFrameSize(1280, 720)
	assert.NoError(t, err)

	width, height = processor.GetFrameSize()
	assert.Equal(t, uint16(1280), width)
	assert.Equal(t, uint16(720), height)

	// Test invalid frame size
	err = processor.SetFrameSize(0, 720)
	assert.Error(t, err)

	err = processor.SetFrameSize(1280, 0)
	assert.Error(t, err)
}

func TestProcessorClose(t *testing.T) {
	processor := NewProcessor()

	err := processor.Close()
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkRealVP8Encoder(b *testing.B) {
	encoder, err := NewRealVP8Encoder(640, 480, 512000)
	if err != nil {
		b.Fatal(err)
	}
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
		_, err := encoder.Encode(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSimpleVP8Encoder(b *testing.B) {
	encoder := NewSimpleVP8Encoder(640, 480, 512000)
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
		_, err := encoder.Encode(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessorProcessOutgoing(b *testing.B) {
	processor := NewProcessor()
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
		_, err := processor.ProcessOutgoing(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessorRoundTrip(b *testing.B) {
	processor := NewProcessor()
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
		data, err := processor.ProcessOutgoingLegacy(frame)
		if err != nil {
			b.Fatal(err)
		}

		_, err = processor.ProcessIncomingLegacy(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
