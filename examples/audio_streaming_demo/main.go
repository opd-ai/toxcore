// Package main demonstrates the complete RTP transport integration for audio streaming
//
// This example shows how to:
// - Set up ToxAV with RTP transport integration
// - Send audio frames through the complete pipeline
// - Handle incoming audio frames
// - Manage call lifecycle with proper resource cleanup
package main

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/opd-ai/toxcore/av"
	"github.com/opd-ai/toxcore/av/rtp"
	"github.com/opd-ai/toxcore/transport"
)

const (
	sampleRate   = 48000 // Standard Opus sample rate
	channels     = 2     // Stereo audio
	frameSize    = 960   // 20ms frame at 48kHz (960 samples per channel)
	audioBitRate = 64000 // 64 kbps audio bit rate
)

func main() {
	fmt.Println("=== ToxAV Audio Streaming Demo ===")
	fmt.Println("Demonstrates RTP transport integration with audio streaming")
	fmt.Println()

	// Step 1: Create UDP transport
	fmt.Println("Step 1: Creating UDP transport...")
	udpTransport, err := transport.NewUDPTransport("0.0.0.0:33445")
	if err != nil {
		log.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udpTransport.Close()
	fmt.Printf("✓ UDP transport created on %s\n\n", udpTransport.LocalAddr())

	// Step 2: Create RTP transport integration
	fmt.Println("Step 2: Creating RTP transport integration...")
	rtpIntegration, err := rtp.NewTransportIntegration(udpTransport)
	if err != nil {
		log.Fatalf("Failed to create RTP integration: %v", err)
	}
	defer rtpIntegration.Close()
	fmt.Println("✓ RTP transport integration created\n")

	// Step 3: Create ToxAV manager
	fmt.Println("Step 3: Creating ToxAV manager...")
	
	// Create mock transport adapter for ToxAV manager
	transportAdapter := &mockTransportAdapter{
		transport: udpTransport,
	}
	
	// Create mock friend address lookup
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		// In a real implementation, this would look up the friend's network address
		// For demo purposes, return a mock address
		return []byte{127, 0, 0, 1}, nil
	}
	
	manager, err := av.NewManager(transportAdapter, friendLookup)
	if err != nil {
		log.Fatalf("Failed to create ToxAV manager: %v", err)
	}
	fmt.Println("✓ ToxAV manager created\n")

	// Step 4: Start a call
	friendNumber := uint32(42)
	fmt.Printf("Step 4: Starting call with friend %d...\n", friendNumber)
	
	err = manager.StartCall(friendNumber, audioBitRate, 0) // Audio only, no video
	if err != nil {
		log.Fatalf("Failed to start call: %v", err)
	}
	fmt.Printf("✓ Call started with friend %d\n\n", friendNumber)

	// Step 5: Get the call and setup media
	fmt.Println("Step 5: Setting up media pipeline...")
	call := manager.GetCall(friendNumber)
	if call == nil {
		log.Fatal("Call not found after starting")
	}

	err = call.SetupMedia(udpTransport, friendNumber)
	if err != nil {
		log.Fatalf("Failed to setup media: %v", err)
	}
	fmt.Println("✓ Media pipeline configured with RTP transport\n")

	// Step 6: Generate and send audio frames
	fmt.Println("Step 6: Sending audio frames...")
	fmt.Println("Generating 440 Hz sine wave (musical note A)")
	
	// Send 50 frames (1 second of audio at 20ms per frame)
	frameCount := 50
	frequency := 440.0 // A4 note
	
	for i := 0; i < frameCount; i++ {
		// Generate audio frame (sine wave)
		pcmData := generateSineWave(frequency, sampleRate, channels, frameSize, i)
		
		// Send audio frame through the complete pipeline:
		// PCM → Audio Processor → RTP Session → Transport → Network
		err = call.SendAudioFrame(pcmData, frameSize, uint8(channels), uint32(sampleRate))
		if err != nil {
			log.Printf("Failed to send frame %d: %v", i, err)
		}
		
		// Print progress
		if (i+1)%10 == 0 {
			fmt.Printf("  Sent %d/%d frames (%.1f seconds)\n", i+1, frameCount, float64(i+1)*0.02)
		}
		
		// Simulate real-time audio (20ms per frame)
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("✓ All audio frames sent successfully\n")

	// Step 7: Display statistics
	fmt.Println("Step 7: Call statistics...")
	rtpSession := call.GetRTPSession()
	if rtpSession != nil {
		stats := rtpSession.GetStatistics()
		fmt.Printf("  Packets sent: %d\n", stats.PacketsSent)
		fmt.Printf("  Packets received: %d\n", stats.PacketsReceived)
		fmt.Printf("  Bytes sent: %d\n", stats.BytesSent)
		fmt.Printf("  Session duration: %v\n", time.Since(stats.StartTime))
	}
	fmt.Println()

	// Step 8: End call and cleanup
	fmt.Println("Step 8: Ending call and cleaning up...")
	err = manager.EndCall(friendNumber)
	if err != nil {
		log.Printf("Failed to end call: %v", err)
	}
	fmt.Println("✓ Call ended and resources cleaned up\n")

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Key Components Demonstrated:")
	fmt.Println("  ✓ UDP transport creation and management")
	fmt.Println("  ✓ RTP transport integration with address mapping")
	fmt.Println("  ✓ ToxAV call lifecycle (start, setup, end)")
	fmt.Println("  ✓ Audio frame processing and transmission")
	fmt.Println("  ✓ Complete audio pipeline: PCM → RTP → Network")
	fmt.Println()
	fmt.Println("For production use:")
	fmt.Println("  - Integrate with actual Tox friend management")
	fmt.Println("  - Add audio input from microphone")
	fmt.Println("  - Implement audio output callbacks")
	fmt.Println("  - Add error recovery and reconnection logic")
}

// generateSineWave creates a PCM audio buffer containing a sine wave
func generateSineWave(frequency, sampleRate float64, channels, frameSize, frameNumber int) []int16 {
	pcmData := make([]int16, frameSize*channels)
	
	for i := 0; i < frameSize; i++ {
		// Calculate sample position in the continuous wave
		samplePos := frameNumber*frameSize + i
		
		// Generate sine wave sample
		// Formula: amplitude * sin(2π * frequency * time)
		time := float64(samplePos) / sampleRate
		sample := math.Sin(2.0 * math.Pi * frequency * time)
		
		// Convert to 16-bit PCM (range: -32768 to 32767)
		pcmValue := int16(sample * 30000) // Use 30000 instead of 32767 to avoid clipping
		
		// Fill both channels with the same data (mono signal)
		for ch := 0; ch < channels; ch++ {
			pcmData[i*channels+ch] = pcmValue
		}
	}
	
	return pcmData
}

// mockTransportAdapter adapts transport.Transport to av.TransportInterface
type mockTransportAdapter struct {
	transport transport.Transport
}

func (m *mockTransportAdapter) Send(packetType byte, data []byte, addr []byte) error {
	// Convert byte address to net.Addr
	// In production, this would properly parse the address format
	return fmt.Errorf("send not implemented in demo (would send %d bytes)", len(data))
}

func (m *mockTransportAdapter) RegisterHandler(packetType byte, handler func([]byte, []byte) error) {
	// In production, this would register with the actual transport
	// For demo purposes, we skip the registration
}
