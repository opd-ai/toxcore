package audio

import (
	"fmt"
	"testing"

	"github.com/pion/opus"
	"github.com/stretchr/testify/assert"
)

func TestNewOpusCodec(t *testing.T) {
	codec := NewOpusCodec()

	assert.NotNil(t, codec)
	assert.NotNil(t, codec.processor)
}

func TestOpusCodecEncodeFrame(t *testing.T) {
	codec := NewOpusCodec()

	pcm := []int16{1000, -1000, 2000, -2000}
	data, err := codec.EncodeFrame(pcm, 48000)

	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, len(pcm)*2, len(data))
}

func TestOpusCodecDecodeFrame(t *testing.T) {
	codec := NewOpusCodec()

	// Test with empty data (error case)
	pcm, sampleRate, err := codec.DecodeFrame([]byte{})
	assert.Error(t, err)
	assert.Nil(t, pcm)
	assert.Equal(t, uint32(0), sampleRate)
}

func TestOpusCodecSetBitRate(t *testing.T) {
	codec := NewOpusCodec()

	err := codec.SetBitRate(96000)
	assert.NoError(t, err)
}

func TestOpusCodecGetSupportedSampleRates(t *testing.T) {
	codec := NewOpusCodec()

	rates := codec.GetSupportedSampleRates()
	expected := []uint32{8000, 12000, 16000, 24000, 48000}

	assert.Equal(t, expected, rates)
}

func TestOpusCodecGetSupportedBitRates(t *testing.T) {
	codec := NewOpusCodec()

	rates := codec.GetSupportedBitRates()
	expected := []uint32{8000, 16000, 32000, 64000, 96000, 128000, 256000, 512000}

	assert.Equal(t, expected, rates)
}

func TestOpusCodecValidateFrameSize(t *testing.T) {
	codec := NewOpusCodec()

	tests := []struct {
		name       string
		frameSize  int
		sampleRate uint32
		channels   int
		expectErr  bool
	}{
		{
			name:       "valid_10ms_mono",
			frameSize:  480, // 10ms at 48kHz
			sampleRate: 48000,
			channels:   1,
			expectErr:  false,
		},
		{
			name:       "valid_20ms_mono",
			frameSize:  960, // 20ms at 48kHz
			sampleRate: 48000,
			channels:   1,
			expectErr:  false,
		},
		{
			name:       "valid_20ms_stereo",
			frameSize:  1920, // 20ms at 48kHz stereo
			sampleRate: 48000,
			channels:   2,
			expectErr:  false,
		},
		{
			name:       "invalid_frame_size",
			frameSize:  500, // ~10.4ms at 48kHz (invalid)
			sampleRate: 48000,
			channels:   1,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := codec.ValidateFrameSize(tt.frameSize, tt.sampleRate, tt.channels)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid Opus frame size")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOpusCodecClose(t *testing.T) {
	codec := NewOpusCodec()

	err := codec.Close()
	assert.NoError(t, err)
}

func TestGetBandwidthFromSampleRate(t *testing.T) {
	tests := []struct {
		sampleRate uint32
		expected   opus.Bandwidth
	}{
		{8000, opus.BandwidthNarrowband},
		{12000, opus.BandwidthMediumband},
		{16000, opus.BandwidthWideband},
		{24000, opus.BandwidthSuperwideband},
		{48000, opus.BandwidthFullband},
		{44100, opus.BandwidthFullband}, // Unsupported rate -> default to fullband
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("rate_%d", tt.sampleRate), func(t *testing.T) {
			bandwidth := GetBandwidthFromSampleRate(tt.sampleRate)
			assert.Equal(t, tt.expected, bandwidth)
		})
	}
}

// Benchmark tests for codec performance
func BenchmarkOpusCodecEncodeFrame(b *testing.B) {
	codec := NewOpusCodec()
	pcm := make([]int16, 960) // 20ms at 48kHz
	for i := range pcm {
		pcm[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.EncodeFrame(pcm, 48000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateFrameSize(b *testing.B) {
	codec := NewOpusCodec()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = codec.ValidateFrameSize(960, 48000, 1)
	}
}
