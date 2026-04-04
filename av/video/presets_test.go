package video

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityPresetString(t *testing.T) {
	tests := []struct {
		preset   QualityPreset
		expected string
	}{
		{QualityLow, "Low"},
		{QualityMedium, "Medium"},
		{QualityHigh, "High"},
		{QualityUltra, "Ultra"},
		{QualityPreset(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.preset.String())
		})
	}
}

func TestGetPresetConfig(t *testing.T) {
	tests := []struct {
		name            string
		preset          QualityPreset
		expectError     bool
		expectedWidth   uint16
		expectedBitrate uint32
	}{
		{
			name:            "Low preset",
			preset:          QualityLow,
			expectError:     false,
			expectedWidth:   320,
			expectedBitrate: 128000,
		},
		{
			name:            "Medium preset",
			preset:          QualityMedium,
			expectError:     false,
			expectedWidth:   640,
			expectedBitrate: 500000,
		},
		{
			name:            "High preset",
			preset:          QualityHigh,
			expectError:     false,
			expectedWidth:   1280,
			expectedBitrate: 1000000,
		},
		{
			name:            "Ultra preset",
			preset:          QualityUltra,
			expectError:     false,
			expectedWidth:   1920,
			expectedBitrate: 4000000,
		},
		{
			name:        "Invalid preset",
			preset:      QualityPreset(99),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetPresetConfig(tt.preset)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedWidth, config.Width)
			assert.Equal(t, tt.expectedBitrate, config.Bitrate)
		})
	}
}

func TestPresetConfigValues(t *testing.T) {
	// Verify all preset configurations are valid and consistent
	presets := AllPresets()
	require.Len(t, presets, 4)

	var prevBitrate uint32
	for _, preset := range presets {
		config, err := GetPresetConfig(preset)
		require.NoError(t, err, "preset %s should have valid config", preset)

		// Dimensions must be even for VP8
		assert.Equal(t, uint16(0), config.Width%2, "width must be even for %s", preset)
		assert.Equal(t, uint16(0), config.Height%2, "height must be even for %s", preset)

		// Bitrate should increase with quality
		assert.Greater(t, config.Bitrate, prevBitrate, "bitrate should increase for %s", preset)
		prevBitrate = config.Bitrate

		// FrameRate must be reasonable
		assert.GreaterOrEqual(t, config.FrameRate, uint8(10), "framerate too low for %s", preset)
		assert.LessOrEqual(t, config.FrameRate, uint8(60), "framerate too high for %s", preset)
	}
}

func TestNewProcessorWithPreset(t *testing.T) {
	tests := []struct {
		name        string
		preset      QualityPreset
		expectError bool
	}{
		{"Low preset processor", QualityLow, false},
		{"Medium preset processor", QualityMedium, false},
		{"High preset processor", QualityHigh, false},
		{"Ultra preset processor", QualityUltra, false},
		{"Invalid preset processor", QualityPreset(99), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := NewProcessorWithPreset(tt.preset)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, processor)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, processor)

			config, _ := GetPresetConfig(tt.preset)
			assert.Equal(t, config.Width, processor.width)
			assert.Equal(t, config.Height, processor.height)
			assert.Equal(t, config.Bitrate, processor.bitRate)

			// Clean up
			err = processor.Close()
			assert.NoError(t, err)
		})
	}
}

func TestPresetForBandwidth(t *testing.T) {
	tests := []struct {
		name          string
		bandwidthKbps uint32
		expected      QualityPreset
	}{
		{"Very low bandwidth", 50, QualityLow},
		{"Low bandwidth", 200, QualityLow},
		{"Medium bandwidth", 800, QualityMedium},
		{"High bandwidth", 1500, QualityHigh},
		{"Very high bandwidth", 6000, QualityUltra},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PresetForBandwidth(tt.bandwidthKbps)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPresetForResolution(t *testing.T) {
	tests := []struct {
		name     string
		width    uint16
		height   uint16
		expected QualityPreset
	}{
		{"QVGA resolution", 320, 240, QualityLow},
		{"VGA resolution", 640, 480, QualityMedium},
		{"HD 720p resolution", 1280, 720, QualityHigh},
		{"Full HD resolution", 1920, 1080, QualityUltra},
		{"4K resolution", 3840, 2160, QualityUltra},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PresetForResolution(tt.width, tt.height)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateBandwidthUsage(t *testing.T) {
	tests := []struct {
		preset   QualityPreset
		expected uint32
	}{
		{QualityLow, 128},      // 128 kbps
		{QualityMedium, 500},   // 500 kbps
		{QualityHigh, 1000},    // 1 Mbps
		{QualityUltra, 4000},   // 4 Mbps
		{QualityPreset(99), 0}, // Invalid preset
	}

	for _, tt := range tests {
		t.Run(tt.preset.String(), func(t *testing.T) {
			result := EstimateBandwidthUsage(tt.preset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePresetForNetwork(t *testing.T) {
	tests := []struct {
		name              string
		preset            QualityPreset
		bandwidthKbps     uint32
		packetLossPercent float32
		expected          bool
	}{
		{
			name:              "Low preset with sufficient bandwidth",
			preset:            QualityLow,
			bandwidthKbps:     200,
			packetLossPercent: 0,
			expected:          true,
		},
		{
			name:              "High preset with insufficient bandwidth",
			preset:            QualityHigh,
			bandwidthKbps:     500,
			packetLossPercent: 0,
			expected:          false,
		},
		{
			name:              "Medium preset with high packet loss",
			preset:            QualityMedium,
			bandwidthKbps:     1000,
			packetLossPercent: 10.0,
			expected:          false,
		},
		{
			name:              "Low preset with high packet loss is still valid",
			preset:            QualityLow,
			bandwidthKbps:     200,
			packetLossPercent: 10.0,
			expected:          true,
		},
		{
			name:              "Ultra preset with ample bandwidth",
			preset:            QualityUltra,
			bandwidthKbps:     10000,
			packetLossPercent: 1.0,
			expected:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePresetForNetwork(tt.preset, tt.bandwidthKbps, tt.packetLossPercent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAllPresets(t *testing.T) {
	presets := AllPresets()
	assert.Len(t, presets, 4)
	assert.Equal(t, QualityLow, presets[0])
	assert.Equal(t, QualityMedium, presets[1])
	assert.Equal(t, QualityHigh, presets[2])
	assert.Equal(t, QualityUltra, presets[3])
}
