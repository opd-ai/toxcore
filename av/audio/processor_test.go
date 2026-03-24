package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProcessor(t *testing.T) {
	processor := NewProcessor()

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.encoder)
	assert.NotNil(t, processor.decoder)
	assert.Equal(t, uint32(48000), processor.sampleRate)
	assert.Equal(t, uint32(64000), processor.bitRate)
}

func TestMagnumOpusEncoder(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate uint32
		bitRate    uint32
		pcm        []int16
		expectErr  bool
	}{
		{
			name:       "valid_encoding_20ms",
			sampleRate: 48000,
			bitRate:    64000,
			pcm:        make([]int16, 960), // 20ms at 48kHz mono
			expectErr:  false,
		},
		{
			name:       "sample_rate_mismatch",
			sampleRate: 44100, // Different from encoder's 48000
			bitRate:    64000,
			pcm:        []int16{1000, -1000},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder, err := NewMagnumOpusEncoder(48000, 64000, 1)
			assert.NoError(t, err)
			assert.NotNil(t, encoder)

			data, err := encoder.Encode(tt.pcm, tt.sampleRate)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				assert.True(t, len(data) > 0)
			}
		})
	}
}

func TestMagnumOpusEncoderSetBitRate(t *testing.T) {
	encoder, err := NewMagnumOpusEncoder(48000, 64000, 1)
	assert.NoError(t, err)

	err = encoder.SetBitRate(96000)
	assert.NoError(t, err)
	assert.Equal(t, uint32(96000), encoder.bitRate)
}

func TestMagnumOpusEncoderClose(t *testing.T) {
	encoder, err := NewMagnumOpusEncoder(48000, 64000, 1)
	assert.NoError(t, err)

	err = encoder.Close()
	assert.NoError(t, err)
}

func TestProcessorProcessOutgoing(t *testing.T) {
	processor := NewProcessor()

	// Test valid processing with 20ms frame
	pcm := make([]int16, 960) // 20ms at 48kHz mono
	for i := range pcm {
		pcm[i] = int16(i % 1000)
	}
	data, err := processor.ProcessOutgoing(pcm, 48000)

	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.True(t, len(data) > 0)
}

func TestProcessorProcessOutgoingError(t *testing.T) {
	processor := &Processor{} // Uninitialized encoder

	pcm := []int16{1000, -1000}
	data, err := processor.ProcessOutgoing(pcm, 48000)

	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "audio encoder not initialized")
}

func TestProcessorProcessIncoming(t *testing.T) {
	processor := NewProcessor()

	// Test empty data
	pcm, sampleRate, err := processor.ProcessIncoming([]byte{})
	assert.Error(t, err)
	assert.Nil(t, pcm)
	assert.Equal(t, uint32(0), sampleRate)
	assert.Contains(t, err.Error(), "empty audio data")

	// Test with invalid data
	invalidData := []byte{0x01, 0x02, 0x03, 0x04}
	pcm, sampleRate, err = processor.ProcessIncoming(invalidData)
	assert.Error(t, err)
	assert.Nil(t, pcm)
	assert.Equal(t, uint32(0), sampleRate)
	assert.Contains(t, err.Error(), "opus decode failed")
}

func TestProcessorRoundTrip(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Create a 20ms frame at 48kHz mono (960 samples)
	pcm := make([]int16, 960)
	for i := range pcm {
		pcm[i] = int16((i % 100) * 100)
	}

	// Encode
	encoded, err := processor.ProcessOutgoing(pcm, 48000)
	assert.NoError(t, err)
	assert.NotNil(t, encoded)

	// Decode
	decoded, sampleRate, err := processor.ProcessIncoming(encoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, uint32(48000), sampleRate)
	assert.True(t, len(decoded) > 0)
}

func TestProcessorSetBitRate(t *testing.T) {
	processor := NewProcessor()

	// Test valid bit rate update
	err := processor.SetBitRate(96000)
	assert.NoError(t, err)
	assert.Equal(t, uint32(96000), processor.bitRate)
}

func TestProcessorSetBitRateError(t *testing.T) {
	processor := &Processor{} // Uninitialized encoder

	err := processor.SetBitRate(96000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audio encoder not initialized")
}

func TestProcessorClose(t *testing.T) {
	processor := NewProcessor()

	err := processor.Close()
	assert.NoError(t, err)
}

func TestProcessorCloseWithNilEncoder(t *testing.T) {
	processor := &Processor{
		encoder: nil,
	}

	err := processor.Close()
	assert.NoError(t, err)
}

// Test resampling integration in ProcessOutgoing
func TestProcessorProcessOutgoingWithResampling(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Test with 8kHz input (should be resampled to 48kHz)
	// 20ms at 8kHz = 160 samples, which after 6x resampling = 960 samples at 48kHz
	pcm := make([]int16, 160)
	for i := range pcm {
		pcm[i] = int16(i * 100)
	}

	output, err := processor.ProcessOutgoing(pcm, 8000)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.True(t, len(output) > 0)

	// Verify resampler was created
	assert.NotNil(t, processor.resampler)
	assert.Equal(t, uint32(8000), processor.resampler.GetInputRate())
	assert.Equal(t, uint32(48000), processor.resampler.GetOutputRate())
}

func TestProcessorProcessOutgoingWithSameRate(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// Test with 48kHz input (should not need resampling)
	pcm := make([]int16, 960) // 20ms of 48kHz audio
	for i := range pcm {
		pcm[i] = int16(i % 1000)
	}

	output, err := processor.ProcessOutgoing(pcm, 48000)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.True(t, len(output) > 0)

	// Verify no resampler was created
	assert.Nil(t, processor.resampler)
}

func TestProcessorProcessOutgoingWithDifferentRates(t *testing.T) {
	processor := NewProcessor()
	defer processor.Close()

	// First call with 16kHz
	pcm16k := make([]int16, 320) // 20ms of 16kHz audio
	output1, err := processor.ProcessOutgoing(pcm16k, 16000)
	assert.NoError(t, err)
	assert.NotNil(t, output1)

	// Verify resampler for 16kHz was created
	assert.NotNil(t, processor.resampler)
	assert.Equal(t, uint32(16000), processor.resampler.GetInputRate())

	// Second call with 44.1kHz (should create new resampler)
	pcm44k := make([]int16, 882) // ~20ms of 44.1kHz audio
	output2, err := processor.ProcessOutgoing(pcm44k, 44100)
	assert.NoError(t, err)
	assert.NotNil(t, output2)

	// Verify resampler was updated for 44.1kHz
	assert.NotNil(t, processor.resampler)
	assert.Equal(t, uint32(44100), processor.resampler.GetInputRate())
}

// Benchmark tests for performance validation
func BenchmarkMagnumOpusEncoder(b *testing.B) {
	encoder, err := NewMagnumOpusEncoder(48000, 64000, 1)
	if err != nil {
		b.Fatal(err)
	}
	pcm := make([]int16, 960) // 20ms of audio at 48kHz
	for i := range pcm {
		pcm[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(pcm, 48000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessorProcessOutgoing(b *testing.B) {
	processor := NewProcessor()
	pcm := make([]int16, 960) // 20ms of audio at 48kHz
	for i := range pcm {
		pcm[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessOutgoing(pcm, 48000)
		if err != nil {
			b.Fatal(err)
		}
	}
}
