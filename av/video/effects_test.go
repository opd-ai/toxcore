package video

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorTemperatureEffect(t *testing.T) {
	tests := []struct {
		name        string
		temperature int
		expectName  string
	}{
		{
			name:        "warm temperature",
			temperature: 50,
			expectName:  "ColorTemperature(Warm+50)",
		},
		{
			name:        "cool temperature",
			temperature: -30,
			expectName:  "ColorTemperature(Cool-30)",
		},
		{
			name:        "neutral temperature",
			temperature: 0,
			expectName:  "ColorTemperature(Neutral)",
		},
		{
			name:        "maximum warm",
			temperature: 100,
			expectName:  "ColorTemperature(Warm+100)",
		},
		{
			name:        "maximum cool",
			temperature: -100,
			expectName:  "ColorTemperature(Cool-100)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect := NewColorTemperatureEffect(tt.temperature)
			frame := createTestFrame(160, 120)

			// Store original values for comparison
			originalU := make([]byte, len(frame.U))
			originalV := make([]byte, len(frame.V))
			copy(originalU, frame.U)
			copy(originalV, frame.V)

			result, err := effect.Apply(frame)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectName, effect.GetName())

			// Y plane should remain unchanged
			assert.Equal(t, frame.Y, result.Y)

			if tt.temperature == 0 {
				// Neutral should not change anything
				assert.Equal(t, originalU, result.U)
				assert.Equal(t, originalV, result.V)
			} else {
				// Temperature adjustment should modify U and V planes
				// For this test, we just verify they're different when temperature != 0
				assert.NotEqual(t, originalU, result.U)
				assert.NotEqual(t, originalV, result.V)
			}
		})
	}
}

func TestColorTemperatureEffect_ClampTemperature(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{150, 100},   // clamp high
		{-150, -100}, // clamp low
		{50, 50},     // no clamp needed
		{0, 0},       // neutral
	}

	for _, tt := range tests {
		effect := NewColorTemperatureEffect(tt.input)
		assert.Equal(t, tt.expected, effect.temperature)
	}
}

func TestColorTemperatureEffect_NilFrame(t *testing.T) {
	effect := NewColorTemperatureEffect(50)
	result, err := effect.Apply(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "input frame cannot be nil")
}

func TestColorTemperatureEffect_ChromaAdjustment(t *testing.T) {
	// Test that color temperature affects U and V planes as expected
	effect := NewColorTemperatureEffect(50) // Warm
	frame := createTestFrame(160, 120)

	// Set specific test values for U and V planes
	for i := range frame.U {
		frame.U[i] = 128 // Neutral chroma
		frame.V[i] = 128 // Neutral chroma
	}

	result, err := effect.Apply(frame)
	require.NoError(t, err)

	// For warm temperature (positive):
	// U should decrease (less blue)
	// V should increase (more red)
	assert.True(t, result.U[0] < frame.U[0], "U should decrease for warm temperature")
	assert.True(t, result.V[0] > frame.V[0], "V should increase for warm temperature")
}

func BenchmarkColorTemperatureEffect(b *testing.B) {
	effect := NewColorTemperatureEffect(50)
	frame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSharpenEffect(b *testing.B) {
	effect := NewSharpenEffect(1.5)
	frame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestNewEffectChain(t *testing.T) {
	chain := NewEffectChain()
	assert.NotNil(t, chain)
	assert.Equal(t, 0, chain.GetEffectCount())
}

func TestEffectChain_AddEffect(t *testing.T) {
	chain := NewEffectChain()
	effect := NewBrightnessEffect(50)

	chain.AddEffect(effect)
	assert.Equal(t, 1, chain.GetEffectCount())

	// Add another effect
	chain.AddEffect(NewContrastEffect(1.5))
	assert.Equal(t, 2, chain.GetEffectCount())
}

func TestEffectChain_Apply_EmptyChain(t *testing.T) {
	chain := NewEffectChain()
	frame := createTestFrame(160, 120)

	result, err := chain.Apply(frame)
	require.NoError(t, err)
	assert.Equal(t, frame.Width, result.Width)
	assert.Equal(t, frame.Height, result.Height)

	// Should be a copy, not the same frame
	frame.Y[0] = 200
	assert.NotEqual(t, byte(200), result.Y[0])
}

func TestEffectChain_Apply_MultipleEffects(t *testing.T) {
	chain := NewEffectChain()
	chain.AddEffect(NewBrightnessEffect(20))
	chain.AddEffect(NewContrastEffect(1.2))
	chain.AddEffect(NewGrayscaleEffect())

	frame := createTestFrame(160, 120)

	result, err := chain.Apply(frame)
	require.NoError(t, err)
	assert.Equal(t, frame.Width, result.Width)
	assert.Equal(t, frame.Height, result.Height)

	// Grayscale should set U and V to 128
	for _, val := range result.U {
		assert.Equal(t, byte(128), val)
	}
	for _, val := range result.V {
		assert.Equal(t, byte(128), val)
	}
}

func TestEffectChain_Apply_NilFrame(t *testing.T) {
	chain := NewEffectChain()
	chain.AddEffect(NewBrightnessEffect(50))

	result, err := chain.Apply(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input frame cannot be nil")
	assert.Nil(t, result)
}

func TestEffectChain_Clear(t *testing.T) {
	chain := NewEffectChain()
	chain.AddEffect(NewBrightnessEffect(50))
	chain.AddEffect(NewContrastEffect(1.5))
	assert.Equal(t, 2, chain.GetEffectCount())

	chain.Clear()
	assert.Equal(t, 0, chain.GetEffectCount())
}

func TestBrightnessEffect(t *testing.T) {
	tests := []struct {
		name       string
		adjustment int
		input      byte
		expected   byte
	}{
		{"increase brightness", 50, 100, 150},
		{"decrease brightness", -50, 150, 100},
		{"clamp high", 200, 200, 255},
		{"clamp low", -200, 50, 0},
		{"no change", 0, 128, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect := NewBrightnessEffect(tt.adjustment)
			frame := createTestFrame(16, 16)
			frame.Y[0] = tt.input

			result, err := effect.Apply(frame)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.Y[0])
		})
	}
}

func TestBrightnessEffect_ClampAdjustment(t *testing.T) {
	// Test extreme values are clamped
	effect1 := NewBrightnessEffect(500)  // Should be clamped to 255
	effect2 := NewBrightnessEffect(-500) // Should be clamped to -255

	assert.Equal(t, "Brightness(+255)", effect1.GetName())
	assert.Equal(t, "Brightness(-255)", effect2.GetName())
}

func TestContrastEffect(t *testing.T) {
	effect := NewContrastEffect(2.0) // High contrast
	frame := createTestFrame(16, 16)

	// Set some test values around midpoint
	frame.Y[0] = 64  // Dark
	frame.Y[1] = 128 // Midpoint (should stay same)
	frame.Y[2] = 192 // Light

	result, err := effect.Apply(frame)
	require.NoError(t, err)

	// High contrast should make dark darker and light lighter
	assert.Less(t, result.Y[0], byte(64))     // Darker
	assert.Equal(t, byte(128), result.Y[1])   // Midpoint unchanged
	assert.Greater(t, result.Y[2], byte(192)) // Lighter
}

func TestContrastEffect_ClampFactor(t *testing.T) {
	// Test extreme values are clamped
	effect1 := NewContrastEffect(5.0)  // Should be clamped to 3.0
	effect2 := NewContrastEffect(-1.0) // Should be clamped to 0.0

	assert.Equal(t, "Contrast(3.00)", effect1.GetName())
	assert.Equal(t, "Contrast(0.00)", effect2.GetName())
}

func TestGrayscaleEffect(t *testing.T) {
	effect := NewGrayscaleEffect()
	frame := createTestFrame(160, 120)

	// Set some chroma values
	for i := range frame.U {
		frame.U[i] = byte(i % 256)
		frame.V[i] = byte((i + 100) % 256)
	}

	result, err := effect.Apply(frame)
	require.NoError(t, err)

	// All chroma should be neutral (128)
	for _, val := range result.U {
		assert.Equal(t, byte(128), val)
	}
	for _, val := range result.V {
		assert.Equal(t, byte(128), val)
	}

	// Y plane should be unchanged
	assert.Equal(t, frame.Y, result.Y)
}

func TestBlurEffect(t *testing.T) {
	effect := NewBlurEffect(1)
	frame := createTestFrame(32, 32)

	// Create a high contrast pattern
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			idx := y*32 + x
			if (x+y)%2 == 0 {
				frame.Y[idx] = 0 // Black
			} else {
				frame.Y[idx] = 255 // White
			}
		}
	}

	result, err := effect.Apply(frame)
	require.NoError(t, err)

	// Blur should reduce contrast (no pure 0 or 255 except at edges)
	centerIdx := 16*32 + 16 // Center pixel
	centerVal := result.Y[centerIdx]
	assert.Greater(t, centerVal, byte(50)) // Should not be pure black
	assert.Less(t, centerVal, byte(200))   // Should not be pure white
}

func TestBlurEffect_ClampRadius(t *testing.T) {
	// Test extreme values are clamped
	effect1 := NewBlurEffect(0)  // Should be clamped to 1
	effect2 := NewBlurEffect(10) // Should be clamped to 5

	assert.Equal(t, "Blur(1)", effect1.GetName())
	assert.Equal(t, "Blur(5)", effect2.GetName())
}

func TestSharpenEffect(t *testing.T) {
	effect := NewSharpenEffect(1.0)
	frame := createTestFrame(32, 32)

	// Create a soft pattern (all mid-gray with one bright pixel)
	for i := range frame.Y {
		frame.Y[i] = 128
	}
	centerIdx := 16*32 + 16
	frame.Y[centerIdx] = 200 // Bright center

	result, err := effect.Apply(frame)
	require.NoError(t, err)

	// Sharpening should enhance the contrast around the bright pixel
	assert.GreaterOrEqual(t, result.Y[centerIdx], frame.Y[centerIdx]) // Center should be same or brighter
}

func TestSharpenEffect_ClampStrength(t *testing.T) {
	// Test extreme values are clamped
	effect1 := NewSharpenEffect(-1.0) // Should be clamped to 0.0
	effect2 := NewSharpenEffect(5.0)  // Should be clamped to 2.0

	assert.Equal(t, "Sharpen(0.00)", effect1.GetName())
	assert.Equal(t, "Sharpen(2.00)", effect2.GetName())
}

func TestEffect_NilFrame(t *testing.T) {
	effects := []Effect{
		NewBrightnessEffect(50),
		NewContrastEffect(1.5),
		NewGrayscaleEffect(),
		NewBlurEffect(1),
		NewSharpenEffect(1.0),
	}

	for _, effect := range effects {
		t.Run(effect.GetName(), func(t *testing.T) {
			result, err := effect.Apply(nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "input frame cannot be nil")
			assert.Nil(t, result)
		})
	}
}

func TestEffectChain_EffectError(t *testing.T) {
	// Create a mock effect that always fails
	chain := NewEffectChain()
	chain.AddEffect(NewBrightnessEffect(50))
	chain.AddEffect(&mockFailingEffect{})

	frame := createTestFrame(160, 120)
	result, err := chain.Apply(frame)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "effect 1 (MockFail) failed")
	assert.Nil(t, result)
}

// Mock effect for testing error handling
type mockFailingEffect struct{}

func (m *mockFailingEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	return nil, fmt.Errorf("mock error")
}

func (m *mockFailingEffect) GetName() string {
	return "MockFail"
}

// Benchmark effect performance
func BenchmarkBrightnessEffect(b *testing.B) {
	effect := NewBrightnessEffect(50)
	frame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContrastEffect(b *testing.B) {
	effect := NewContrastEffect(1.5)
	frame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBlurEffect(b *testing.B) {
	effect := NewBlurEffect(2)
	frame := createTestFrame(320, 240) // Smaller frame for blur benchmark

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := effect.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEffectChain_Multiple(b *testing.B) {
	chain := NewEffectChain()
	chain.AddEffect(NewBrightnessEffect(20))
	chain.AddEffect(NewContrastEffect(1.2))
	chain.AddEffect(NewGrayscaleEffect())

	frame := createTestFrame(640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chain.Apply(frame)
		if err != nil {
			b.Fatal(err)
		}
	}
}
