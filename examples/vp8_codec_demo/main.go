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
	printDemoHeader()
	codec := initializeCodec()
	defer codec.Close()

	displaySupportedResolutions(codec)
	testResolutions := defineTestResolutions()
	testAllResolutions(codec, testResolutions)
	displayPerformanceSummary()
}

// printDemoHeader displays the application title
func printDemoHeader() {
	fmt.Println("VP8 Codec Demo Application")
	fmt.Println("=========================")
}

// initializeCodec creates and returns a new VP8 codec instance
func initializeCodec() *video.VP8Codec {
	return video.NewVP8Codec()
}

// displaySupportedResolutions shows all codec-supported resolutions with bitrates
func displaySupportedResolutions(codec *video.VP8Codec) {
	fmt.Println("\nSupported Resolutions:")
	resolutions := codec.GetSupportedResolutions()
	for _, res := range resolutions {
		bitrate := video.GetBitrateForResolution(res)
		fmt.Printf("  %s - Recommended bitrate: %d bps (%.1f Mbps)\n",
			res.String(), bitrate, float64(bitrate)/1000000)
	}
}

// defineTestResolutions returns the list of resolutions to test
func defineTestResolutions() []video.Resolution {
	return []video.Resolution{
		{Width: 320, Height: 240},  // QVGA
		{Width: 640, Height: 480},  // VGA
		{Width: 1280, Height: 720}, // HD 720p
	}
}

// testAllResolutions runs encoding/decoding tests for all specified resolutions
func testAllResolutions(codec *video.VP8Codec, testResolutions []video.Resolution) {
	fmt.Println("\nTesting different resolutions:")
	for _, res := range testResolutions {
		testSingleResolution(codec, res)
	}
}

// testSingleResolution performs a complete encode/decode test for one resolution
func testSingleResolution(codec *video.VP8Codec, res video.Resolution) {
	fmt.Printf("\nTesting %s:\n", res.String())

	if !validateResolution(codec, res) {
		return
	}

	processor := createProcessorForResolution(res)
	defer processor.Close()

	if !configureProcessorBitrate(processor, res) {
		return
	}

	frame := createAndDisplayTestFrame(res)
	encodeTime, data := encodeFrameWithTiming(processor, frame)
	if data == nil {
		return
	}

	decodeTime, decodedFrame := decodeFrameWithTiming(processor, data)
	if decodedFrame == nil {
		return
	}

	verifyAndDisplayIntegrity(frame, decodedFrame)
	displayRoundTripMetrics(encodeTime, decodeTime)
}

// validateResolution checks if the resolution is supported by the codec
func validateResolution(codec *video.VP8Codec, res video.Resolution) bool {
	err := codec.ValidateFrameSize(res.Width, res.Height)
	if err != nil {
		log.Printf("  ❌ Validation failed: %v", err)
		return false
	}
	fmt.Printf("  ✅ Frame size validation passed\n")
	return true
}

// createProcessorForResolution creates a video processor with appropriate settings
func createProcessorForResolution(res video.Resolution) *video.Processor {
	bitrate := video.GetBitrateForResolution(res)
	return video.NewProcessorWithSettings(res.Width, res.Height, bitrate)
}

// configureProcessorBitrate sets the bitrate for the processor
func configureProcessorBitrate(processor *video.Processor, res video.Resolution) bool {
	bitrate := video.GetBitrateForResolution(res)
	err := processor.SetBitRate(bitrate)
	if err != nil {
		log.Printf("  ❌ Bitrate setting failed: %v", err)
		return false
	}
	fmt.Printf("  ✅ Bitrate set to %d bps\n", bitrate)
	return true
}

// createAndDisplayTestFrame creates a test frame and displays its metadata
func createAndDisplayTestFrame(res video.Resolution) *video.VideoFrame {
	frame := createTestFrame(res.Width, res.Height)
	fmt.Printf("  📋 Created test frame: %dx%d (%d bytes)\n",
		frame.Width, frame.Height, len(frame.Y)+len(frame.U)+len(frame.V))
	return frame
}

// encodeFrameWithTiming encodes a frame and measures encoding time
func encodeFrameWithTiming(processor *video.Processor, frame *video.VideoFrame) (time.Duration, []byte) {
	start := time.Now()
	data, err := processor.ProcessOutgoingLegacy(frame)
	encodeTime := time.Since(start)

	if err != nil {
		log.Printf("  ❌ Encoding failed: %v", err)
		return 0, nil
	}
	fmt.Printf("  ⚡ Encoded in %v (%d bytes)\n", encodeTime, len(data))
	return encodeTime, data
}

// decodeFrameWithTiming decodes frame data and measures decoding time
func decodeFrameWithTiming(processor *video.Processor, data []byte) (time.Duration, *video.VideoFrame) {
	start := time.Now()
	decodedFrame, err := processor.ProcessIncomingLegacy(data)
	decodeTime := time.Since(start)

	if err != nil {
		log.Printf("  ❌ Decoding failed: %v", err)
		return 0, nil
	}
	fmt.Printf("  ⚡ Decoded in %v\n", decodeTime)
	return decodeTime, decodedFrame
}

// verifyAndDisplayIntegrity verifies frame integrity and displays result
func verifyAndDisplayIntegrity(original, decoded *video.VideoFrame) {
	if verifyFrameIntegrity(original, decoded) {
		fmt.Printf("  ✅ Frame integrity verified\n")
	} else {
		fmt.Printf("  ❌ Frame integrity check failed\n")
	}
}

// displayRoundTripMetrics calculates and displays round-trip performance metrics
func displayRoundTripMetrics(encodeTime, decodeTime time.Duration) {
	totalTime := encodeTime + decodeTime
	fps := time.Second / totalTime
	fmt.Printf("  📊 Round-trip time: %v (max FPS: %.1f)\n", totalTime, float64(fps))
}

// displayPerformanceSummary runs and displays comprehensive performance tests
func displayPerformanceSummary() {
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
	if !validateFrameDimensions(original, decoded) {
		return false
	}

	if !validateBufferLengths(original, decoded) {
		return false
	}

	if !verifyYPlane(original, decoded) {
		return false
	}

	if !verifyUPlane(original, decoded) {
		return false
	}

	if !verifyVPlane(original, decoded) {
		return false
	}

	return true
}

// validateFrameDimensions checks if frame dimensions match between original and decoded frames.
func validateFrameDimensions(original, decoded *video.VideoFrame) bool {
	return original.Width == decoded.Width && original.Height == decoded.Height
}

// validateBufferLengths verifies that all buffer lengths match between original and decoded frames.
func validateBufferLengths(original, decoded *video.VideoFrame) bool {
	return len(original.Y) == len(decoded.Y) &&
		len(original.U) == len(decoded.U) &&
		len(original.V) == len(decoded.V)
}

// verifyYPlane compares the Y (luminance) plane data between original and decoded frames.
func verifyYPlane(original, decoded *video.VideoFrame) bool {
	for i := range original.Y {
		if original.Y[i] != decoded.Y[i] {
			return false
		}
	}
	return true
}

// verifyUPlane compares the U (chrominance) plane data between original and decoded frames.
func verifyUPlane(original, decoded *video.VideoFrame) bool {
	for i := range original.U {
		if original.U[i] != decoded.U[i] {
			return false
		}
	}
	return true
}

// verifyVPlane compares the V (chrominance) plane data between original and decoded frames.
func verifyVPlane(original, decoded *video.VideoFrame) bool {
	for i := range original.V {
		if original.V[i] != decoded.V[i] {
			return false
		}
	}
	return true
}

// runPerformanceTest measures performance with different frame sizes
func runPerformanceTest() {
	tests := definePerformanceTests()
	iterations := 100

	for _, test := range tests {
		runSinglePerformanceTest(test, iterations)
	}

	fmt.Println("✅ All performance tests completed successfully!")
}

// definePerformanceTests returns the list of performance test configurations
func definePerformanceTests() []struct {
	name string
	res  video.Resolution
} {
	return []struct {
		name string
		res  video.Resolution
	}{
		{"QVGA", video.Resolution{Width: 320, Height: 240}},
		{"VGA", video.Resolution{Width: 640, Height: 480}},
		{"HD", video.Resolution{Width: 1280, Height: 720}},
	}
}

// runSinglePerformanceTest executes a complete performance test for one resolution
func runSinglePerformanceTest(test struct {
	name string
	res  video.Resolution
}, iterations int,
) {
	processor := createPerformanceProcessor(test.res)
	defer processor.Close()

	frame := createTestFrame(test.res.Width, test.res.Height)
	warmUpProcessor(processor, frame)

	encodeAvg := measureEncodePerformance(processor, frame, iterations)
	decodeAvg, dataSize := measureDecodePerformance(processor, frame, iterations)

	displayPerformanceResults(test.name, test.res, encodeAvg, decodeAvg, dataSize)
}

// createPerformanceProcessor creates a processor configured for performance testing
func createPerformanceProcessor(res video.Resolution) *video.Processor {
	return video.NewProcessorWithSettings(res.Width, res.Height,
		video.GetBitrateForResolution(res))
}

// warmUpProcessor performs warm-up iterations to stabilize performance measurements
func warmUpProcessor(processor *video.Processor, frame *video.VideoFrame) {
	for i := 0; i < 10; i++ {
		processor.ProcessOutgoing(frame)
	}
}

// measureEncodePerformance measures average encoding time over multiple iterations
func measureEncodePerformance(processor *video.Processor, frame *video.VideoFrame, iterations int) time.Duration {
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := processor.ProcessOutgoing(frame)
		if err != nil {
			log.Printf("Encoding error: %v", err)
			continue
		}
	}
	return time.Since(start) / time.Duration(iterations)
}

// measureDecodePerformance measures average decoding time and returns data size
func measureDecodePerformance(processor *video.Processor, frame *video.VideoFrame, iterations int) (time.Duration, int) {
	data, _ := processor.ProcessOutgoingLegacy(frame)
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := processor.ProcessIncomingLegacy(data)
		if err != nil {
			log.Printf("Decoding error: %v", err)
			continue
		}
	}
	decodeAvg := time.Since(start) / time.Duration(iterations)
	return decodeAvg, len(data)
}

// displayPerformanceResults calculates and displays performance metrics
func displayPerformanceResults(name string, res video.Resolution, encodeAvg, decodeAvg time.Duration, dataSize int) {
	totalAvg := encodeAvg + decodeAvg
	maxFPS := time.Second / totalAvg

	fmt.Printf("%s (%s):\n", name, res.String())
	fmt.Printf("  Encode: %v\n", encodeAvg)
	fmt.Printf("  Decode: %v\n", decodeAvg)
	fmt.Printf("  Total:  %v\n", totalAvg)
	fmt.Printf("  Max FPS: %.1f\n", float64(maxFPS))
	fmt.Printf("  Data size: %d bytes\n", dataSize)
	fmt.Println()
}
