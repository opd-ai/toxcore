// VP8 Codec Demo Application
//
// This example demonstrates the VP8 codec functionality including:
// - Video frame creation and processing
// - Encoding and decoding operations
// - Resolution and bitrate management
// - Performance measurement
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore/av/video"
)

func main() {
	fmt.Println("VP8 Codec Demo Application")
	fmt.Println("=========================")

	// Create VP8 codec instance
	codec := video.NewVP8Codec()
	defer codec.Close()

	// Demonstrate supported resolutions
	fmt.Println("\nSupported Resolutions:")
	resolutions := codec.GetSupportedResolutions()
	for _, res := range resolutions {
		bitrate := video.GetBitrateForResolution(res)
		fmt.Printf("  %s - Recommended bitrate: %d bps (%.1f Mbps)\n",
			res.String(), bitrate, float64(bitrate)/1000000)
	}

	// Test different frame sizes
	testResolutions := []video.Resolution{
		{Width: 320, Height: 240},  // QVGA
		{Width: 640, Height: 480},  // VGA
		{Width: 1280, Height: 720}, // HD 720p
	}

	fmt.Println("\nTesting different resolutions:")

	for _, res := range testResolutions {
		fmt.Printf("\nTesting %s:\n", res.String())

		// Validate frame size
		err := codec.ValidateFrameSize(res.Width, res.Height)
		if err != nil {
			log.Printf("  ❌ Validation failed: %v", err)
			continue
		}
		fmt.Printf("  ✅ Frame size validation passed\n")

		// Create processor with the specific resolution for this test
		processor := video.NewProcessorWithSettings(res.Width, res.Height, video.GetBitrateForResolution(res))
		defer processor.Close()

		// Set appropriate bitrate
		bitrate := video.GetBitrateForResolution(res)
		err = processor.SetBitRate(bitrate)
		if err != nil {
			log.Printf("  ❌ Bitrate setting failed: %v", err)
			continue
		}
		fmt.Printf("  ✅ Bitrate set to %d bps\n", bitrate)

		// Create test frame
		frame := createTestFrame(res.Width, res.Height)
		fmt.Printf("  📋 Created test frame: %dx%d (%d bytes)\n",
			frame.Width, frame.Height, len(frame.Y)+len(frame.U)+len(frame.V))

		// Measure encoding performance
		start := time.Now()
		data, err := processor.ProcessOutgoingLegacy(frame)
		encodeTime := time.Since(start)

		if err != nil {
			log.Printf("  ❌ Encoding failed: %v", err)
			continue
		}
		fmt.Printf("  ⚡ Encoded in %v (%d bytes)\n", encodeTime, len(data))

		// Measure decoding performance
		start = time.Now()
		decodedFrame, err := processor.ProcessIncomingLegacy(data)
		decodeTime := time.Since(start)

		if err != nil {
			log.Printf("  ❌ Decoding failed: %v", err)
			continue
		}
		fmt.Printf("  ⚡ Decoded in %v\n", decodeTime)

		// Verify integrity
		if verifyFrameIntegrity(frame, decodedFrame) {
			fmt.Printf("  ✅ Frame integrity verified\n")
		} else {
			fmt.Printf("  ❌ Frame integrity check failed\n")
		}

		// Calculate throughput
		totalTime := encodeTime + decodeTime
		fps := time.Second / totalTime
		fmt.Printf("  📊 Round-trip time: %v (max FPS: %.1f)\n", totalTime, float64(fps))
	} // Performance summary
	fmt.Println("\nPerformance Summary:")
	fmt.Println("==================")
	runPerformanceTest()
}

// createTestFrame generates a test video frame with a simple pattern
func createTestFrame(width, height uint16) *video.VideoFrame {
	ySize := int(width) * int(height)
	uvSize := ySize / 4

	frame := &video.VideoFrame{
		Width:   width,
		Height:  height,
		Y:       make([]byte, ySize),
		U:       make([]byte, uvSize),
		V:       make([]byte, uvSize),
		YStride: int(width),
		UStride: int(width) / 2,
		VStride: int(width) / 2,
	}

	// Create a gradient pattern in Y (luminance)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			// Create a diagonal gradient
			value := byte((x + y) % 256)
			frame.Y[y*int(width)+x] = value
		}
	}

	// Create patterns in U and V (chrominance)
	for i := range frame.U {
		frame.U[i] = byte((i * 2) % 256)
		frame.V[i] = byte((i * 3) % 256)
	}

	return frame
}

// verifyFrameIntegrity checks if decoded frame matches the original
func verifyFrameIntegrity(original, decoded *video.VideoFrame) bool {
	if original.Width != decoded.Width || original.Height != decoded.Height {
		return false
	}

	if len(original.Y) != len(decoded.Y) ||
		len(original.U) != len(decoded.U) ||
		len(original.V) != len(decoded.V) {
		return false
	}

	// Check Y plane
	for i := range original.Y {
		if original.Y[i] != decoded.Y[i] {
			return false
		}
	}

	// Check U plane
	for i := range original.U {
		if original.U[i] != decoded.U[i] {
			return false
		}
	}

	// Check V plane
	for i := range original.V {
		if original.V[i] != decoded.V[i] {
			return false
		}
	}

	return true
}

// runPerformanceTest measures performance with different frame sizes
func runPerformanceTest() {
	tests := []struct {
		name string
		res  video.Resolution
	}{
		{"QVGA", video.Resolution{Width: 320, Height: 240}},
		{"VGA", video.Resolution{Width: 640, Height: 480}},
		{"HD", video.Resolution{Width: 1280, Height: 720}},
	}

	iterations := 100

	for _, test := range tests {
		// Create processor for this specific resolution
		processor := video.NewProcessorWithSettings(test.res.Width, test.res.Height,
			video.GetBitrateForResolution(test.res))
		defer processor.Close()

		frame := createTestFrame(test.res.Width, test.res.Height)

		// Warm up
		for i := 0; i < 10; i++ {
			processor.ProcessOutgoing(frame)
		}

		// Measure encoding
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, err := processor.ProcessOutgoing(frame)
			if err != nil {
				log.Printf("Encoding error: %v", err)
				continue
			}
		}
		encodeAvg := time.Since(start) / time.Duration(iterations)

		// Measure round-trip
		data, _ := processor.ProcessOutgoingLegacy(frame)
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_, err := processor.ProcessIncomingLegacy(data)
			if err != nil {
				log.Printf("Decoding error: %v", err)
				continue
			}
		}
		decodeAvg := time.Since(start) / time.Duration(iterations)

		totalAvg := encodeAvg + decodeAvg
		maxFPS := time.Second / totalAvg

		fmt.Printf("%s (%s):\n", test.name, test.res.String())
		fmt.Printf("  Encode: %v\n", encodeAvg)
		fmt.Printf("  Decode: %v\n", decodeAvg)
		fmt.Printf("  Total:  %v\n", totalAvg)
		fmt.Printf("  Max FPS: %.1f\n", float64(maxFPS))
		fmt.Printf("  Data size: %d bytes\n", len(data))
		fmt.Println()
	}

	fmt.Println("✅ All performance tests completed successfully!")
}
