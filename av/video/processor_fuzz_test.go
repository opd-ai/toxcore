//go:build go1.18
// +build go1.18

package video

import (
	"testing"
)

// FuzzVP8FrameTag fuzzes the VP8 frame tag parser to ensure it doesn't panic
// on arbitrary input data.
func FuzzVP8FrameTag(f *testing.F) {
	// Seed corpus with valid VP8 frame patterns
	// Key frame: bit 0 = 0, valid start code 0x9D012A at offset 3
	keyFrame := []byte{0x00, 0x00, 0x00, 0x9D, 0x01, 0x2A, 0x00, 0x00, 0x00, 0x00}
	// Inter frame: bit 0 = 1
	interFrame := []byte{0x01, 0x00, 0x00, 0x00}
	// Minimal 3-byte header
	minimal := []byte{0x00, 0x00, 0x00}
	// Empty
	empty := []byte{}
	// Short (1 byte)
	short := []byte{0x00}
	// Invalid start code for key frame
	invalidStart := []byte{0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00}

	f.Add(keyFrame)
	f.Add(interFrame)
	f.Add(minimal)
	f.Add(empty)
	f.Add(short)
	f.Add(invalidStart)

	f.Fuzz(func(t *testing.T, data []byte) {
		// These functions should never panic regardless of input
		vp8FrameTag(data)
		isVP8KeyFrame(data)
		isVP8InterFrame(data)
	})
}

// FuzzDecodeFrameData fuzzes the video frame decoder to ensure it handles
// malformed input gracefully without panicking.
func FuzzDecodeFrameData(f *testing.F) {
	// Create a processor for testing
	proc, err := NewProcessor(160, 120, 100000)
	if err != nil {
		f.Fatalf("Failed to create processor: %v", err)
	}
	defer proc.Stop()

	// Seed corpus with various VP8 patterns
	// Valid-looking key frame header (but will fail decode due to invalid payload)
	keyFrameHeader := []byte{
		0x00, 0x00, 0x00, // Frame tag (key frame, first partition size 0)
		0x9D, 0x01, 0x2A, // VP8 start code
		0xA0, 0x00, // Width (160 in little-endian VP8 format)
		0x78, 0x00, // Height (120 in little-endian VP8 format)
	}

	// Inter frame header
	interFrameHeader := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	// Random bytes
	random := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}

	// Very short
	tooShort := []byte{0x00, 0x01}

	// All zeros
	zeros := make([]byte, 100)

	// All ones
	ones := make([]byte, 100)
	for i := range ones {
		ones[i] = 0xFF
	}

	f.Add(keyFrameHeader)
	f.Add(interFrameHeader)
	f.Add(random)
	f.Add(tooShort)
	f.Add(zeros)
	f.Add(ones)

	f.Fuzz(func(t *testing.T, data []byte) {
		// Create a fresh processor for each fuzz iteration to avoid state
		// from previous iterations affecting results.
		proc, err := NewProcessor(160, 120, 100000)
		if err != nil {
			return // Can't create processor, skip this iteration
		}
		defer proc.Stop()

		// decodeFrameData should never panic; errors are acceptable
		_, _ = proc.decodeFrameData(data)
	})
}

// FuzzDecodeKeyFrame fuzzes the key frame decoder directly.
func FuzzDecodeKeyFrame(f *testing.F) {
	// Valid VP8 key frame header structure
	validHeader := []byte{
		0x00, 0x00, 0x00, // Frame tag
		0x9D, 0x01, 0x2A, // VP8 start code
		0xA0, 0x00, // Width
		0x78, 0x00, // Height
	}

	// Append some payload bytes
	withPayload := append(validHeader, make([]byte, 100)...)

	f.Add(validHeader)
	f.Add(withPayload)
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		proc, err := NewProcessor(160, 120, 100000)
		if err != nil {
			return
		}
		defer proc.Stop()

		// decodeKeyFrame should never panic; errors are acceptable
		_, _ = proc.decodeKeyFrame(data)
	})
}
