// Package video provides video scaling capabilities for ToxAV.
//
// This file implements video frame scaling functionality to resize
// YUV420 frames between different resolutions while maintaining
// aspect ratio and quality.
package video

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// Scaler provides video frame scaling functionality.
//
// Implements efficient YUV420 frame scaling using bilinear interpolation
// for smooth resizing while maintaining the aspect ratio and color quality.
type Scaler struct {
	// No fields needed for stateless scaling operations
}

// NewScaler creates a new video frame scaler.
func NewScaler() *Scaler {
	logrus.WithFields(logrus.Fields{
		"function": "NewScaler",
	}).Info("Creating new video scaler")

	scaler := &Scaler{}

	logrus.WithFields(logrus.Fields{
		"function": "NewScaler",
	}).Info("Video scaler created successfully")

	return scaler
}

// Scale resizes a YUV420 video frame to the specified dimensions.
//
// Uses bilinear interpolation for high-quality scaling while maintaining
// the YUV420 format structure. Both width and height must be even numbers
// to maintain proper YUV420 chroma subsampling.
//
// Parameters:
//   - frame: Source video frame to scale
//   - targetWidth: Target width (must be even and >= 16)
//   - targetHeight: Target height (must be even and >= 16)
//
// Returns:
//   - *VideoFrame: Scaled video frame in YUV420 format
//   - error: Any error that occurred during scaling
func (s *Scaler) Scale(frame *VideoFrame, targetWidth, targetHeight uint16) (*VideoFrame, error) {
	if err := s.validateScaleInputs(frame, targetWidth, targetHeight); err != nil {
		return nil, err
	}

	// If dimensions are the same, return a copy
	if frame.Width == targetWidth && frame.Height == targetHeight {
		return s.copyFrame(frame), nil
	}

	// Create output frame with proper dimensions
	result := s.createOutputFrame(targetWidth, targetHeight)

	// Scale all planes
	if err := s.scaleAllPlanes(frame, result, targetWidth, targetHeight); err != nil {
		return nil, err
	}

	return result, nil
}

// validateScaleInputs validates the input frame and target dimensions for scaling operations.
// Returns an error if any validation check fails.
func (s *Scaler) validateScaleInputs(frame *VideoFrame, targetWidth, targetHeight uint16) error {
	if frame == nil {
		return fmt.Errorf("source frame cannot be nil")
	}

	// Validate target dimensions
	if targetWidth == 0 || targetHeight == 0 {
		return fmt.Errorf("invalid target dimensions: %dx%d", targetWidth, targetHeight)
	}

	if targetWidth%2 != 0 || targetHeight%2 != 0 {
		return fmt.Errorf("target dimensions must be even for YUV420: %dx%d", targetWidth, targetHeight)
	}

	if targetWidth < 16 || targetHeight < 16 {
		return fmt.Errorf("target dimensions too small: %dx%d (minimum 16x16)", targetWidth, targetHeight)
	}

	return nil
}

// copyFrame creates a deep copy of the input video frame.
// Used when scaling is not required (same dimensions).
func (s *Scaler) copyFrame(frame *VideoFrame) *VideoFrame {
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

// createOutputFrame allocates and initializes a new video frame with the specified target dimensions.
// Calculates appropriate strides and buffer sizes for YUV420 format.
func (s *Scaler) createOutputFrame(targetWidth, targetHeight uint16) *VideoFrame {
	// Calculate scaled plane sizes
	ySize := int(targetWidth) * int(targetHeight)
	uvWidth := targetWidth / 2
	uvHeight := targetHeight / 2
	uvSize := int(uvWidth) * int(uvHeight)

	// Create output frame
	return &VideoFrame{
		Width:   targetWidth,
		Height:  targetHeight,
		YStride: int(targetWidth),
		UStride: int(uvWidth),
		VStride: int(uvWidth),
		Y:       make([]byte, ySize),
		U:       make([]byte, uvSize),
		V:       make([]byte, uvSize),
	}
}

// scaleAllPlanes coordinates the scaling of Y, U, and V planes for YUV420 format.
// Handles both luminance (Y) and chroma (U, V) planes with appropriate dimensions.
func (s *Scaler) scaleAllPlanes(source, dest *VideoFrame, targetWidth, targetHeight uint16) error {
	// Scale Y plane (luminance)
	err := s.scalePlane(source.Y, source.Width, source.Height, source.YStride,
		dest.Y, targetWidth, targetHeight, dest.YStride)
	if err != nil {
		return fmt.Errorf("failed to scale Y plane: %v", err)
	}

	// Calculate chroma dimensions
	srcUVWidth := source.Width / 2
	srcUVHeight := source.Height / 2
	uvWidth := targetWidth / 2
	uvHeight := targetHeight / 2

	// Scale U plane (chroma)
	err = s.scalePlane(source.U, srcUVWidth, srcUVHeight, source.UStride,
		dest.U, uvWidth, uvHeight, dest.UStride)
	if err != nil {
		return fmt.Errorf("failed to scale U plane: %v", err)
	}

	// Scale V plane (chroma)
	err = s.scalePlane(source.V, srcUVWidth, srcUVHeight, source.VStride,
		dest.V, uvWidth, uvHeight, dest.VStride)
	if err != nil {
		return fmt.Errorf("failed to scale V plane: %v", err)
	}

	return nil
}

// scalePlane scales a single plane using bilinear interpolation.
//
// This is an internal helper method that performs the actual pixel
// interpolation for individual Y, U, or V planes.
func (s *Scaler) scalePlane(src []byte, srcWidth, srcHeight uint16, srcStride int,
	dst []byte, dstWidth, dstHeight uint16, dstStride int,
) error {
	if len(src) < int(srcHeight)*srcStride {
		return fmt.Errorf("source buffer too small: %d < %d", len(src), int(srcHeight)*srcStride)
	}

	if len(dst) < int(dstHeight)*dstStride {
		return fmt.Errorf("destination buffer too small: %d < %d", len(dst), int(dstHeight)*dstStride)
	}

	// Calculate scaling ratios
	xRatio := float64(srcWidth) / float64(dstWidth)
	yRatio := float64(srcHeight) / float64(dstHeight)

	// Bilinear interpolation
	for y := uint16(0); y < dstHeight; y++ {
		for x := uint16(0); x < dstWidth; x++ {
			// Calculate source coordinates
			srcX := float64(x) * xRatio
			srcY := float64(y) * yRatio

			// Get integer and fractional parts
			x1 := int(srcX)
			y1 := int(srcY)
			x2 := x1 + 1
			y2 := y1 + 1

			// Clamp to bounds
			if x2 >= int(srcWidth) {
				x2 = int(srcWidth) - 1
			}
			if y2 >= int(srcHeight) {
				y2 = int(srcHeight) - 1
			}

			// Get fractional parts
			fx := srcX - float64(x1)
			fy := srcY - float64(y1)

			// Sample source pixels
			p11 := float64(src[y1*srcStride+x1])
			p12 := float64(src[y1*srcStride+x2])
			p21 := float64(src[y2*srcStride+x1])
			p22 := float64(src[y2*srcStride+x2])

			// Bilinear interpolation
			top := p11*(1-fx) + p12*fx
			bottom := p21*(1-fx) + p22*fx
			pixel := top*(1-fy) + bottom*fy

			// Store result
			dst[int(y)*dstStride+int(x)] = byte(pixel + 0.5) // Round to nearest
		}
	}

	return nil
}

// GetScaleFactors calculates the scaling factors for given dimensions.
//
// Utility function to determine how much a frame will be scaled.
//
// Parameters:
//   - srcWidth, srcHeight: Source frame dimensions
//   - dstWidth, dstHeight: Target frame dimensions
//
// Returns:
//   - xFactor: Horizontal scaling factor
//   - yFactor: Vertical scaling factor
func (s *Scaler) GetScaleFactors(srcWidth, srcHeight, dstWidth, dstHeight uint16) (xFactor, yFactor float64) {
	xFactor = float64(dstWidth) / float64(srcWidth)
	yFactor = float64(dstHeight) / float64(srcHeight)
	return xFactor, yFactor
}

// IsScalingRequired checks if scaling is needed for given dimensions.
func (s *Scaler) IsScalingRequired(srcWidth, srcHeight, dstWidth, dstHeight uint16) bool {
	return srcWidth != dstWidth || srcHeight != dstHeight
}
