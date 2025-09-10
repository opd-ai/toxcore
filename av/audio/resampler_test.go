package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResampler(t *testing.T) {
	tests := []struct {
		name      string
		config    ResamplerConfig
		expectErr bool
	}{
		{
			name: "valid_config",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   2,
				Quality:    4,
			},
			expectErr: false,
		},
		{
			name: "zero_input_rate",
			config: ResamplerConfig{
				InputRate:  0,
				OutputRate: 48000,
				Channels:   1,
				Quality:    4,
			},
			expectErr: true,
		},
		{
			name: "zero_output_rate",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 0,
				Channels:   1,
				Quality:    4,
			},
			expectErr: true,
		},
		{
			name: "invalid_channels_zero",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   0,
				Quality:    4,
			},
			expectErr: true,
		},
		{
			name: "invalid_channels_three",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   3,
				Quality:    4,
			},
			expectErr: true,
		},
		{
			name: "invalid_quality_negative",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   1,
				Quality:    -1,
			},
			expectErr: true,
		},
		{
			name: "invalid_quality_too_high",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   1,
				Quality:    11,
			},
			expectErr: true,
		},
		{
			name: "default_quality",
			config: ResamplerConfig{
				InputRate:  44100,
				OutputRate: 48000,
				Channels:   1,
				Quality:    0, // Should default to 4
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resampler, err := NewResampler(tt.config)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, resampler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resampler)
				assert.Equal(t, tt.config.InputRate, resampler.GetInputRate())
				assert.Equal(t, tt.config.OutputRate, resampler.GetOutputRate())
				assert.Equal(t, tt.config.Channels, resampler.GetChannels())

				expectedQuality := tt.config.Quality
				if expectedQuality == 0 {
					expectedQuality = 4
				}
				assert.Equal(t, expectedQuality, resampler.GetQuality())
			}
		})
	}
}

func TestResamplerSameRate(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	input := []int16{100, 200, 300, 400, 500}
	output, err := resampler.Resample(input)

	assert.NoError(t, err)
	assert.Equal(t, input, output)
}

func TestResamplerUpsample(t *testing.T) {
	// Test upsampling from 8kHz to 16kHz (2x)
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  8000,
		OutputRate: 16000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	// Simple input signal
	input := []int16{1000, 2000, 3000, 4000}
	output, err := resampler.Resample(input)

	assert.NoError(t, err)
	assert.True(t, len(output) >= len(input)*2-1) // Should be approximately 2x
	assert.True(t, len(output) <= len(input)*2+1) // Allow for rounding
}

func TestResamplerDownsample(t *testing.T) {
	// Test downsampling from 48kHz to 24kHz (0.5x)
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: 24000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	// Create a longer input to ensure proper downsampling
	input := make([]int16, 96) // 2ms of 48kHz audio
	for i := range input {
		input[i] = int16(i * 100)
	}

	output, err := resampler.Resample(input)

	assert.NoError(t, err)
	assert.True(t, len(output) >= len(input)/2-2) // Should be approximately 0.5x
	assert.True(t, len(output) <= len(input)/2+2) // Allow for rounding
}

func TestResamplerStereo(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   2,
		Quality:    4,
	})
	require.NoError(t, err)

	// Stereo input: L, R, L, R, ...
	input := []int16{1000, 2000, 1100, 2100, 1200, 2200, 1300, 2300}
	output, err := resampler.Resample(input)

	assert.NoError(t, err)
	assert.True(t, len(output)%2 == 0) // Output should be stereo-aligned
	assert.True(t, len(output) > 0)
}

func TestResamplerInvalidInput(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	// Test empty input
	output, err := resampler.Resample([]int16{})
	assert.Error(t, err)
	assert.Nil(t, output)
}

func TestResamplerChannelAlignment(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   2,
		Quality:    4,
	})
	require.NoError(t, err)

	// Input not aligned to channel count (odd number for stereo)
	input := []int16{1000, 2000, 3000} // 3 samples for 2 channels
	output, err := resampler.Resample(input)

	assert.Error(t, err)
	assert.Nil(t, output)
}

func TestResamplerCalculateOutputSize(t *testing.T) {
	tests := []struct {
		name       string
		inputRate  uint32
		outputRate uint32
		inputSize  int
		expected   int
	}{
		{
			name:       "same_rate",
			inputRate:  48000,
			outputRate: 48000,
			inputSize:  100,
			expected:   100,
		},
		{
			name:       "upsample_2x",
			inputRate:  24000,
			outputRate: 48000,
			inputSize:  100,
			expected:   200,
		},
		{
			name:       "downsample_0.5x",
			inputRate:  48000,
			outputRate: 24000,
			inputSize:  100,
			expected:   50,
		},
		{
			name:       "cd_to_opus",
			inputRate:  44100,
			outputRate: 48000,
			inputSize:  441, // 10ms of CD audio
			expected:   480, // Approximately 10ms of Opus audio
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resampler, err := NewResampler(ResamplerConfig{
				InputRate:  tt.inputRate,
				OutputRate: tt.outputRate,
				Channels:   1,
				Quality:    4,
			})
			require.NoError(t, err)

			result := resampler.CalculateOutputSize(tt.inputSize)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResamplerReset(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	// Process some data to change internal state
	input := []int16{1000, 2000, 3000, 4000}
	_, err = resampler.Resample(input)
	require.NoError(t, err)

	// Reset should not return an error
	err = resampler.Reset()
	assert.NoError(t, err)

	// Internal state should be reset (position should be 0)
	assert.Equal(t, 0.0, resampler.position)
}

func TestResamplerClose(t *testing.T) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  44100,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	err = resampler.Close()
	assert.NoError(t, err)
}

func TestResamplerContinuity(t *testing.T) {
	// Test that resampling multiple chunks maintains continuity
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  8000,
		OutputRate: 16000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(t, err)

	// Generate a sine wave input
	const frequency = 1000.0    // 1kHz tone
	chunk1 := make([]int16, 80) // 10ms at 8kHz
	chunk2 := make([]int16, 80) // Another 10ms

	for i := range chunk1 {
		t := float64(i) / 8000.0
		chunk1[i] = int16(10000 * math.Sin(2*math.Pi*frequency*t))
	}

	for i := range chunk2 {
		t := float64(80+i) / 8000.0
		chunk2[i] = int16(10000 * math.Sin(2*math.Pi*frequency*t))
	}

	output1, err := resampler.Resample(chunk1)
	require.NoError(t, err)

	output2, err := resampler.Resample(chunk2)
	require.NoError(t, err)

	// Both outputs should have reasonable sizes
	assert.True(t, len(output1) > 0)
	assert.True(t, len(output2) > 0)
}

// Test the convenience functions
func TestTelephoneToOpusResampler(t *testing.T) {
	resampler, err := NewTelephoneToOpusResampler(1)
	require.NoError(t, err)

	assert.Equal(t, uint32(8000), resampler.GetInputRate())
	assert.Equal(t, uint32(48000), resampler.GetOutputRate())
	assert.Equal(t, 1, resampler.GetChannels())
}

func TestCDToOpusResampler(t *testing.T) {
	resampler, err := NewCDToOpusResampler(2)
	require.NoError(t, err)

	assert.Equal(t, uint32(44100), resampler.GetInputRate())
	assert.Equal(t, uint32(48000), resampler.GetOutputRate())
	assert.Equal(t, 2, resampler.GetChannels())
}

func TestWidebandToOpusResampler(t *testing.T) {
	resampler, err := NewWidebandToOpusResampler(1)
	require.NoError(t, err)

	assert.Equal(t, uint32(16000), resampler.GetInputRate())
	assert.Equal(t, uint32(48000), resampler.GetOutputRate())
	assert.Equal(t, 1, resampler.GetChannels())
}

func TestOpusToPlaybackResampler(t *testing.T) {
	resampler, err := NewOpusToPlaybackResampler(44100, 2)
	require.NoError(t, err)

	assert.Equal(t, uint32(48000), resampler.GetInputRate())
	assert.Equal(t, uint32(44100), resampler.GetOutputRate())
	assert.Equal(t, 2, resampler.GetChannels())
}

// Benchmark tests for performance validation
func BenchmarkResamplerSameRate(b *testing.B) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(b, err)

	input := make([]int16, 480) // 10ms of 48kHz audio
	for i := range input {
		input[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resampler.Resample(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResamplerUpsample(b *testing.B) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  8000,
		OutputRate: 48000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(b, err)

	input := make([]int16, 80) // 10ms of 8kHz audio
	for i := range input {
		input[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resampler.Resample(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResamplerDownsample(b *testing.B) {
	resampler, err := NewResampler(ResamplerConfig{
		InputRate:  48000,
		OutputRate: 8000,
		Channels:   1,
		Quality:    4,
	})
	require.NoError(b, err)

	input := make([]int16, 480) // 10ms of 48kHz audio
	for i := range input {
		input[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resampler.Resample(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResamplerCDToOpus(b *testing.B) {
	resampler, err := NewCDToOpusResampler(2)
	require.NoError(b, err)

	input := make([]int16, 882) // 10ms of CD audio, stereo
	for i := range input {
		input[i] = int16(i % 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resampler.Resample(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}
