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
	"net"
	"strconv"
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

	udpTransport := createUDPTransport()
	defer udpTransport.Close()

	rtpIntegration := createRTPIntegration(udpTransport)
	defer rtpIntegration.Close()

	manager := createToxAVManager(udpTransport)
	friendNumber := uint32(42)

	startCall(manager, friendNumber)
	call := setupMediaPipeline(manager, friendNumber, udpTransport)
	sendAudioFrames(call)
	displayCallStatistics(call)
	endCall(manager, friendNumber)
	displaySummary()
}

func createUDPTransport() transport.Transport {
	fmt.Println("Step 1: Creating UDP transport...")
	udpTransport, err := transport.NewUDPTransport("0.0.0.0:33445")
	if err != nil {
		log.Fatalf("Failed to create UDP transport: %v", err)
	}
	fmt.Printf("✓ UDP transport created on %s\n\n", udpTransport.LocalAddr())
	return udpTransport
}

func createRTPIntegration(udpTransport transport.Transport) *rtp.TransportIntegration {
	fmt.Println("Step 2: Creating RTP transport integration...")
	rtpIntegration, err := rtp.NewTransportIntegration(udpTransport)
	if err != nil {
		log.Fatalf("Failed to create RTP integration: %v", err)
	}
	fmt.Println("✓ RTP transport integration created")
	fmt.Println()
	return rtpIntegration
}

func createToxAVManager(udpTransport transport.Transport) *av.Manager {
	fmt.Println("Step 3: Creating ToxAV manager...")

	transportAdapter := &mockTransportAdapter{transport: udpTransport}
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{127, 0, 0, 1}, nil
	}

	manager, err := av.NewManager(transportAdapter, friendLookup)
	if err != nil {
		log.Fatalf("Failed to create ToxAV manager: %v", err)
	}
	fmt.Println("✓ ToxAV manager created")
	fmt.Println()
	return manager
}

func startCall(manager *av.Manager, friendNumber uint32) {
	fmt.Printf("Step 4: Starting call with friend %d...\n", friendNumber)
	err := manager.StartCall(friendNumber, audioBitRate, 0)
	if err != nil {
		log.Fatalf("Failed to start call: %v", err)
	}
	fmt.Printf("✓ Call started with friend %d\n\n", friendNumber)
}

func setupMediaPipeline(manager *av.Manager, friendNumber uint32, udpTransport transport.Transport) *av.Call {
	fmt.Println("Step 5: Setting up media pipeline...")
	call := manager.GetCall(friendNumber)
	if call == nil {
		log.Fatal("Call not found after starting")
	}

	err := call.SetupMedia(udpTransport, friendNumber)
	if err != nil {
		log.Fatalf("Failed to setup media: %v", err)
	}
	fmt.Println("✓ Media pipeline configured with RTP transport")
	fmt.Println()
	return call
}

func sendAudioFrames(call *av.Call) {
	fmt.Println("Step 6: Sending audio frames...")
	fmt.Println("Generating 440 Hz sine wave (musical note A)")

	frameCount := 50
	frequency := 440.0

	for i := 0; i < frameCount; i++ {
		pcmData := generateSineWave(frequency, sampleRate, channels, frameSize, i)
		err := call.SendAudioFrame(pcmData, frameSize, uint8(channels), uint32(sampleRate))
		if err != nil {
			log.Printf("Failed to send frame %d: %v", i, err)
		}

		if (i+1)%10 == 0 {
			fmt.Printf("  Sent %d/%d frames (%.1f seconds)\n", i+1, frameCount, float64(i+1)*0.02)
		}

		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("✓ All audio frames sent successfully")
	fmt.Println()
}

func displayCallStatistics(call *av.Call) {
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
}

func endCall(manager *av.Manager, friendNumber uint32) {
	fmt.Println("Step 8: Ending call and cleaning up...")
	err := manager.EndCall(friendNumber)
	if err != nil {
		log.Printf("Failed to end call: %v", err)
	}
	fmt.Println("✓ Call ended and resources cleaned up")
	fmt.Println()
}

func displaySummary() {
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

// avPacketTypeMap maps AV packet type bytes to transport.PacketType values.
var avPacketTypeMap = map[byte]transport.PacketType{
	0x30: transport.PacketAVCallRequest,
	0x31: transport.PacketAVCallResponse,
	0x32: transport.PacketAVCallControl,
	0x33: transport.PacketAVAudioFrame,
	0x34: transport.PacketAVVideoFrame,
	0x35: transport.PacketAVBitrateControl,
}

// Send transmits a packet through the underlying UDP transport.
// The byte address format is 4 bytes IPv4 + 2 bytes port (big-endian).
func (m *mockTransportAdapter) Send(packetType byte, data, addr []byte) error {
	pt, ok := avPacketTypeMap[packetType]
	if !ok {
		return fmt.Errorf("unknown AV packet type: 0x%02x", packetType)
	}
	if len(addr) != 6 {
		return fmt.Errorf("address must be exactly 6 bytes (IPv4+port), got %d", len(addr))
	}
	ip := fmt.Sprintf("%d.%d.%d.%d:%d", addr[0], addr[1], addr[2], addr[3],
		int(addr[4])<<8|int(addr[5]))
	netAddr, err := net.ResolveUDPAddr("udp4", ip)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", ip, err)
	}
	return m.transport.Send(&transport.Packet{PacketType: pt, Data: data}, netAddr)
}

// RegisterHandler registers a packet handler with the underlying UDP transport.
func (m *mockTransportAdapter) RegisterHandler(packetType byte, handler func([]byte, []byte) error) {
	pt, ok := avPacketTypeMap[packetType]
	if !ok {
		return
	}
	m.transport.RegisterHandler(pt, func(packet *transport.Packet, addr net.Addr) error {
		addrStr := addr.String()
		host, portStr, err := net.SplitHostPort(addrStr)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", addrStr, err)
		}
		ip := net.ParseIP(host).To4()
		if ip == nil {
			return fmt.Errorf("non-IPv4 address not supported in demo: %s", host)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("failed to parse port %q: %w", portStr, err)
		}
		if port < 0 || port > 65535 {
			return fmt.Errorf("port out of range: %d", port)
		}
		addrBytes := []byte{ip[0], ip[1], ip[2], ip[3], byte(port >> 8), byte(port)}
		return handler(packet.Data, addrBytes)
	})
}
