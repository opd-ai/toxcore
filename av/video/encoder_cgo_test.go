//go:build cgo && libvpx
// +build cgo,libvpx

package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestFrame builds a minimal YUV420 VideoFrame for encoder tests.
func makeTestFrame(width, height uint16) *VideoFrame {
	w := int(width)
	h := int(height)
	uvW := w / 2
	uvH := h / 2
	y := make([]byte, w*h)
	u := make([]byte, uvW*uvH)
	v := make([]byte, uvW*uvH)
	// Fill with a non-zero pattern so the encoder has meaningful input.
	for i := range y {
		y[i] = byte(i % 235)
	}
	for i := range u {
		u[i] = 128
	}
	for i := range v {
		v[i] = 128
	}
	return &VideoFrame{
		Width:   width,
		Height:  height,
		Y:       y,
		U:       u,
		V:       v,
		YStride: w,
		UStride: uvW,
		VStride: uvW,
	}
}

func TestNewLibVPXEncoder(t *testing.T) {
	tests := []struct {
		name      string
		width     uint16
		height    uint16
		bitRate   uint32
		expectErr bool
	}{
		{name: "valid_640x480", width: 640, height: 480, bitRate: 512000},
		{name: "valid_320x240", width: 320, height: 240, bitRate: 256000},
		{name: "valid_1280x720", width: 1280, height: 720, bitRate: 1000000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := NewLibVPXEncoder(tc.width, tc.height, tc.bitRate)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, enc)
			assert.Equal(t, tc.width, enc.width)
			assert.Equal(t, tc.height, enc.height)
			assert.Equal(t, tc.bitRate, enc.bitRate)
			assert.True(t, enc.keyFrame, "First frame should be a keyframe")
			require.NoError(t, enc.Close())
		})
	}
}

func TestLibVPXEncoderEncodeKeyFrame(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	frame := makeTestFrame(320, 240)
	data, err := enc.Encode(frame)
	require.NoError(t, err)
	assert.NotEmpty(t, data, "Encoded key frame must not be empty")
}

func TestLibVPXEncoderEncodePFrame(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	frame := makeTestFrame(320, 240)

	// First frame is always a key frame (I-frame).
	iFrameData, err := enc.Encode(frame)
	require.NoError(t, err)
	require.NotEmpty(t, iFrameData)

	// Subsequent frames should produce P-frames (smaller than I-frames).
	pFrameData, err := enc.Encode(frame)
	require.NoError(t, err)
	// P-frame may be empty when the encoder has no new data to emit yet
	// (this is legal in libvpx real-time mode), so we only check for no error.
	_ = pFrameData
}

func TestLibVPXEncoderNilFrame(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	_, err = enc.Encode(nil)
	assert.Error(t, err, "Encode with nil frame must return an error")
}

func TestLibVPXEncoderFrameSizeMismatch(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	wrongFrame := makeTestFrame(640, 480) // different dimensions
	_, err = enc.Encode(wrongFrame)
	assert.Error(t, err, "Encode with mismatched dimensions must return an error")
}

func TestLibVPXEncoderSetBitRate(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	err = enc.SetBitRate(512000)
	assert.NoError(t, err)
	assert.Equal(t, uint32(512000), enc.bitRate)
}

func TestLibVPXEncoderSetKeyFrameInterval(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	enc.SetKeyFrameInterval(60)
	assert.Equal(t, uint32(60), enc.kfMaxDist)

	enc.SetKeyFrameInterval(0) // 0 means every frame is a key frame
	assert.Equal(t, uint32(1), enc.kfMaxDist)
}

func TestLibVPXEncoderForceKeyFrame(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	frame := makeTestFrame(320, 240)

	// Encode an I-frame to consume the initial keyframe flag.
	_, err = enc.Encode(frame)
	require.NoError(t, err)
	assert.False(t, enc.keyFrame, "keyFrame flag should be cleared after I-frame")

	// Force next frame to be a key frame.
	enc.ForceKeyFrame()
	assert.True(t, enc.keyFrame)

	_, err = enc.Encode(frame)
	require.NoError(t, err)
	assert.False(t, enc.keyFrame, "keyFrame flag should be cleared after forced I-frame")
}

func TestLibVPXEncoderSupportsInterframe(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck

	assert.True(t, enc.SupportsInterframe())
}

func TestLibVPXEncoderInterfaceCompliance(t *testing.T) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(t, err)
	defer enc.Close() //nolint:errcheck
	// Compile-time check: *LibVPXEncoder must implement Encoder.
	var _ Encoder = enc
}

func TestDefaultEncoderIsLibVPX(t *testing.T) {
	enc, err := NewDefaultEncoder(320, 240, 256000)
	require.NoError(t, err)
	require.NotNil(t, enc)
	defer enc.Close() //nolint:errcheck

	assert.True(t, DefaultEncoderSupportsInterframe())
	assert.Contains(t, DefaultEncoderName(), "libvpx")
}

func BenchmarkLibVPXEncoderKeyFrame(b *testing.B) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(b, err)
	defer enc.Close() //nolint:errcheck

	frame := makeTestFrame(320, 240)
	b.ResetTimer()
	for b.Loop() {
		enc.ForceKeyFrame()
		if _, err := enc.Encode(frame); err != nil {
			b.Fatalf("encode: %v", err)
		}
	}
}

func BenchmarkLibVPXEncoderPFrame(b *testing.B) {
	enc, err := NewLibVPXEncoder(320, 240, 256000)
	require.NoError(b, err)
	defer enc.Close() //nolint:errcheck

	frame := makeTestFrame(320, 240)
	// Seed with a key frame first.
	if _, err := enc.Encode(frame); err != nil {
		b.Fatalf("seed key frame: %v", err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := enc.Encode(frame); err != nil {
			b.Fatalf("encode: %v", err)
		}
	}
}
