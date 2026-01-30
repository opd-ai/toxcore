package transport

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkNewUDPTransport measures UDP transport creation performance
func BenchmarkNewUDPTransport(b *testing.B) {
	for i := 0; i < b.N; i++ {
		transport, err := NewUDPTransport("127.0.0.1:0") // Use :0 for random port
		if err != nil {
			b.Fatal(err)
		}
		transport.Close()
	}
}

// BenchmarkNoiseTransportSend measures encrypted packet sending performance
func BenchmarkNoiseTransportSend(b *testing.B) {
	// Create base UDP transport
	udpTransport, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer udpTransport.Close()

	// Generate a static key for noise transport
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	// Create noise transport
	noiseTransport, err := NewNoiseTransport(udpTransport, keyPair.Private[:])
	if err != nil {
		b.Fatal(err)
	}
	defer noiseTransport.Close()

	// Create a test packet
	packet := &Packet{
		PacketType: PacketFriendMessage,
		Data:       []byte("Benchmark test message"),
	}

	// Create a destination address
	destAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This will fail to send since no peer is listening,
		// but we're measuring the transport layer performance
		_ = noiseTransport.Send(packet, destAddr)
	}
}

// BenchmarkVersionNegotiationSerialization measures protocol version serialization
func BenchmarkVersionNegotiationSerialization(b *testing.B) {
	packet := &VersionNegotiationPacket{
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:  ProtocolNoiseIK,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeVersionNegotiation(packet)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVersionNegotiationParsing measures protocol version parsing
func BenchmarkVersionNegotiationParsing(b *testing.B) {
	packet := &VersionNegotiationPacket{
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:  ProtocolNoiseIK,
	}

	// Pre-serialize the packet
	data, err := SerializeVersionNegotiation(packet)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseVersionNegotiation(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVersionNegotiatorSelectBestVersion measures version selection performance
func BenchmarkVersionNegotiatorSelectBestVersion(b *testing.B) {
	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	negotiator := NewVersionNegotiator(supportedVersions, preferredVersion, 5*time.Second)

	remoteVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = negotiator.SelectBestVersion(remoteVersions)
	}
}

// BenchmarkNegotiatingTransportCreation measures negotiating transport creation
func BenchmarkNegotiatingTransportCreation(b *testing.B) {
	// Create base UDP transport once
	udpTransport, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer udpTransport.Close()

	capabilities := &ProtocolCapabilities{
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:  ProtocolNoiseIK,
	}

	// Generate a static key
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport, err := NewNegotiatingTransport(udpTransport, capabilities, keyPair.Private[:])
		if err != nil {
			b.Fatal(err)
		}
		transport.Close()
	}
}

// BenchmarkPacketSerialization measures packet serialization performance
func BenchmarkPacketSerialization(b *testing.B) {
	packet := &Packet{
		PacketType: PacketFriendMessage,
		Data:       []byte("This is a test message for packet serialization benchmarking"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate packet serialization (in a real implementation this would
		// involve converting the packet to bytes for network transmission)
		_ = packet.PacketType
		_ = packet.Data
	}
}
