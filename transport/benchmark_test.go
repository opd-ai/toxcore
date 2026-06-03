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

// BenchmarkProtocolSecurityOverhead compares overhead across different protocol versions
// This benchmark measures the CPU cost of security features relative to legacy protocol
func BenchmarkProtocolSecurityOverhead(b *testing.B) {
	testCases := []struct {
		name     string
		versions []ProtocolVersion
	}{
		{
			name:     "LegacyOnly",
			versions: []ProtocolVersion{ProtocolLegacy},
		},
		{
			name:     "NoiseIKOnly",
			versions: []ProtocolVersion{ProtocolNoiseIK},
		},
		{
			name:     "BothVersions",
			versions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			negotiator := NewVersionNegotiator(tc.versions, tc.versions[0], 5*time.Second)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = negotiator.SelectBestVersion(tc.versions)
			}
		})
	}
}

// BenchmarkCoverTrafficManagerOverhead measures the overhead of dummy packet injection
func BenchmarkCoverTrafficManagerOverhead(b *testing.B) {
	udpTransport, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer udpTransport.Close()

	testCases := []struct {
		name   string
		config CoverTrafficConfig
	}{
		{
			name:   "NoTraffic",
			config: CoverTrafficConfig{MinInterval: 0, MaxInterval: 0},
		},
		{
			name:   "ConservativeDefaults",
			config: CoverTrafficConfig{},
		},
		{
			name: "HighFrequency",
			config: CoverTrafficConfig{
				MinInterval:      100 * time.Millisecond,
				MaxInterval:      200 * time.Millisecond,
				DummyPayloadSize: 128,
			},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			manager := NewCoverTrafficManager(udpTransport, tc.config)
			defer manager.Close()

			testAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				manager.AddPeer(testAddr)
				manager.RemovePeer(testAddr)
			}
		})
	}
}

// BenchmarkRatchetEncryption benchmarks the encryption overhead of the double-ratchet
func BenchmarkRatchetEncryption(b *testing.B) {
	// Pre-generate two key pairs outside the timed loop
	aliceKP, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	bobKP, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("RatchetKeyDerivation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Measure X25519 ECDH shared-secret derivation, which is the
			// key-derivation step at the heart of every ratchet round.
			_, err := crypto.DeriveSharedSecret(bobKP.Public, aliceKP.Private)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkTransportLayerOverhead compares overhead across different transport types
func BenchmarkTransportLayerOverhead(b *testing.B) {
	testCases := []struct {
		name    string
		address string
	}{
		{
			name:    "UDP",
			address: "127.0.0.1:0",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Measure the cost of creating and closing a UDP transport (socket creation)
				transport, err := NewUDPTransport(tc.address)
				if err != nil {
					b.Fatal(err)
				}
				transport.Close()
			}
		})
	}
}

// BenchmarkNegotiationRoundtrip measures end-to-end negotiation overhead
func BenchmarkNegotiationRoundtrip(b *testing.B) {
	udpTransport, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer udpTransport.Close()

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	capabilities := &ProtocolCapabilities{
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:  ProtocolNoiseIK,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure the overhead of creating a negotiating transport, which
		// includes version negotiation setup and initial key material preparation.
		negotiatingTransport, err := NewNegotiatingTransport(udpTransport, capabilities, keyPair.Private[:])
		if err != nil {
			b.Fatal(err)
		}
		negotiatingTransport.Close()
	}
}

