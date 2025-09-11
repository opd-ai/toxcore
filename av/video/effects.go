// Package video provides video effects processing capabilities for ToxAV.
//
// This file implements basic video effects that can be applied to
// YUV420 frames including brightness, contrast, and basic filters.
package video

import (
	"fmt"
)

// Effect represents a video effect that can be applied to frames.
type Effect interface {
	// Apply processes a video frame and returns the modified frame
	Apply(frame *VideoFrame) (*VideoFrame, error)
	// GetName returns the effect name for identification
	GetName() string
}

// EffectChain manages multiple effects applied in sequence.
type EffectChain struct {
	effects []Effect
}

// NewEffectChain creates a new effect processing chain.
func NewEffectChain() *EffectChain {
	return &EffectChain{
		effects: make([]Effect, 0),
	}
}

// AddEffect adds an effect to the processing chain.
func (ec *EffectChain) AddEffect(effect Effect) {
	ec.effects = append(ec.effects, effect)
}

// Apply processes a frame through all effects in the chain.
func (ec *EffectChain) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	// If no effects, return a copy
	if len(ec.effects) == 0 {
		return copyFrame(frame), nil
	}

	// Process through effect chain
	current := copyFrame(frame)
	for i, effect := range ec.effects {
		result, err := effect.Apply(current)
		if err != nil {
			return nil, fmt.Errorf("effect %d (%s) failed: %v", i, effect.GetName(), err)
		}
		current = result
	}

	return current, nil
}

// GetEffectCount returns the number of effects in the chain.
func (ec *EffectChain) GetEffectCount() int {
	return len(ec.effects)
}

// Clear removes all effects from the chain.
func (ec *EffectChain) Clear() {
	ec.effects = ec.effects[:0]
}

// BrightnessEffect adjusts the brightness of video frames.
type BrightnessEffect struct {
	adjustment int // -255 to +255
}

// NewBrightnessEffect creates a brightness adjustment effect.
// adjustment: -255 (darkest) to +255 (brightest), 0 = no change
func NewBrightnessEffect(adjustment int) *BrightnessEffect {
	// Clamp to valid range
	if adjustment < -255 {
		adjustment = -255
	}
	if adjustment > 255 {
		adjustment = 255
	}

	return &BrightnessEffect{
		adjustment: adjustment,
	}
}

// Apply adjusts the brightness of the Y (luminance) plane.
func (be *BrightnessEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	result := copyFrame(frame)

	// Adjust Y plane only (luminance)
	for i, pixel := range result.Y {
		newValue := int(pixel) + be.adjustment

		// Clamp to 0-255 range
		if newValue < 0 {
			newValue = 0
		} else if newValue > 255 {
			newValue = 255
		}

		result.Y[i] = byte(newValue)
	}

	return result, nil
}

// GetName returns the effect name.
func (be *BrightnessEffect) GetName() string {
	return fmt.Sprintf("Brightness(%+d)", be.adjustment)
}

// ContrastEffect adjusts the contrast of video frames.
type ContrastEffect struct {
	factor float64 // 0.0 = gray, 1.0 = normal, 2.0 = high contrast
}

// NewContrastEffect creates a contrast adjustment effect.
// factor: 0.0 (no contrast/gray) to 3.0 (high contrast), 1.0 = no change
func NewContrastEffect(factor float64) *ContrastEffect {
	// Clamp to reasonable range
	if factor < 0.0 {
		factor = 0.0
	}
	if factor > 3.0 {
		factor = 3.0
	}

	return &ContrastEffect{
		factor: factor,
	}
}

// Apply adjusts the contrast of the Y (luminance) plane.
func (ce *ContrastEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	result := copyFrame(frame)

	// Adjust Y plane contrast around midpoint (128)
	const midpoint = 128.0

	for i, pixel := range result.Y {
		// Apply contrast around midpoint
		newValue := midpoint + (float64(pixel)-midpoint)*ce.factor

		// Clamp to 0-255 range
		if newValue < 0 {
			newValue = 0
		} else if newValue > 255 {
			newValue = 255
		}

		result.Y[i] = byte(newValue + 0.5) // Round to nearest
	}

	return result, nil
}

// GetName returns the effect name.
func (ce *ContrastEffect) GetName() string {
	return fmt.Sprintf("Contrast(%.2f)", ce.factor)
}

// GrayscaleEffect converts frames to grayscale by zeroing chroma planes.
type GrayscaleEffect struct{}

// NewGrayscaleEffect creates a grayscale conversion effect.
func NewGrayscaleEffect() *GrayscaleEffect {
	return &GrayscaleEffect{}
}

// Apply converts the frame to grayscale by setting U and V to neutral.
func (ge *GrayscaleEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	result := copyFrame(frame)

	// Set chroma planes to neutral (128) for grayscale
	for i := range result.U {
		result.U[i] = 128
	}
	for i := range result.V {
		result.V[i] = 128
	}

	return result, nil
}

// GetName returns the effect name.
func (ge *GrayscaleEffect) GetName() string {
	return "Grayscale"
}

// BlurEffect applies a simple box blur to the luminance plane.
type BlurEffect struct {
	radius int // Blur radius (1-5)
}

// NewBlurEffect creates a blur effect with specified radius.
// radius: 1-5, larger values create more blur
func NewBlurEffect(radius int) *BlurEffect {
	// Clamp to reasonable range
	if radius < 1 {
		radius = 1
	}
	if radius > 5 {
		radius = 5
	}

	return &BlurEffect{
		radius: radius,
	}
}

// Apply applies box blur to the Y (luminance) plane.
func (ble *BlurEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	result := copyFrame(frame)

	// Apply box blur to Y plane
	width := int(frame.Width)
	height := int(frame.Height)

	// Create temporary buffer for blur calculation
	temp := make([]byte, len(result.Y))
	copy(temp, result.Y)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sum := 0
			count := 0

			// Sample pixels in radius
			for dy := -ble.radius; dy <= ble.radius; dy++ {
				for dx := -ble.radius; dx <= ble.radius; dx++ {
					nx := x + dx
					ny := y + dy

					// Check bounds
					if nx >= 0 && nx < width && ny >= 0 && ny < height {
						sum += int(temp[ny*width+nx])
						count++
					}
				}
			}

			// Set blurred value
			if count > 0 {
				result.Y[y*width+x] = byte(sum / count)
			}
		}
	}

	return result, nil
}

// GetName returns the effect name.
func (ble *BlurEffect) GetName() string {
	return fmt.Sprintf("Blur(%d)", ble.radius)
}

// SharpenEffect applies a simple sharpening filter to the luminance plane.
type SharpenEffect struct {
	strength float64 // 0.0 = no effect, 1.0 = normal, 2.0 = strong
}

// NewSharpenEffect creates a sharpening effect with specified strength.
// strength: 0.0 to 2.0, higher values create more sharpening
func NewSharpenEffect(strength float64) *SharpenEffect {
	// Clamp to reasonable range
	if strength < 0.0 {
		strength = 0.0
	}
	if strength > 2.0 {
		strength = 2.0
	}

	return &SharpenEffect{
		strength: strength,
	}
}

// Apply applies unsharp mask sharpening to the Y (luminance) plane.
func (se *SharpenEffect) Apply(frame *VideoFrame) (*VideoFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("input frame cannot be nil")
	}

	result := copyFrame(frame)

	// Simple sharpening kernel: center weighted positively, neighbors negatively
	width := int(frame.Width)
	height := int(frame.Height)

	// Create temporary buffer
	temp := make([]byte, len(result.Y))
	copy(temp, result.Y)

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			idx := y*width + x

			// Apply 3x3 sharpening kernel
			center := float64(temp[idx])
			sum := center * (1.0 + 4.0*se.strength)

			// Subtract neighbors
			sum -= float64(temp[(y-1)*width+x]) * se.strength // Top
			sum -= float64(temp[(y+1)*width+x]) * se.strength // Bottom
			sum -= float64(temp[y*width+(x-1)]) * se.strength // Left
			sum -= float64(temp[y*width+(x+1)]) * se.strength // Right

			// Clamp and store
			if sum < 0 {
				sum = 0
			} else if sum > 255 {
				sum = 255
			}

			result.Y[idx] = byte(sum + 0.5) // Round to nearest
		}
	}

	return result, nil
}

// GetName returns the effect name.
func (se *SharpenEffect) GetName() string {
	return fmt.Sprintf("Sharpen(%.2f)", se.strength)
}

// copyFrame creates a deep copy of a video frame.
func copyFrame(frame *VideoFrame) *VideoFrame {
	return &VideoFrame{
		Width:   frame.Width,
		Height:  frame.Height,
		YStride: frame.YStride,
		UStride: frame.UStride,
		VStride: frame.VStride,
		Y:       append([]byte(nil), frame.Y...),
		U:       append([]byte(nil), frame.U...),
		V:       append([]byte(nil), frame.V...),
	}
}
