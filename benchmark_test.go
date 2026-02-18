package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// BenchmarkNewTox measures the performance of creating a new Tox instance
func BenchmarkNewTox(b *testing.B) {
	options := NewOptions()
	options.UDPEnabled = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tox, err := New(options)
		if err != nil {
			b.Fatal(err)
		}
		tox.Kill()
	}
}

// BenchmarkToxFromSavedata measures savedata restoration performance
func BenchmarkToxFromSavedata(b *testing.B) {
	// Create a Tox instance with some data to get realistic savedata
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}

	err = tox.SelfSetName("Benchmark User")
	if err != nil {
		b.Fatal(err)
	}

	savedata := tox.GetSavedata()
	tox.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tox, err := NewFromSavedata(options, savedata)
		if err != nil {
			b.Fatal(err)
		}
		tox.Kill()
	}
}

// BenchmarkSelfSetName measures name setting performance
func BenchmarkSelfSetName(b *testing.B) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tox.SelfSetName("Benchmark Test Name")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAddFriendByPublicKey measures friend addition performance
func BenchmarkAddFriendByPublicKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			b.Fatal(err)
		}

		// Generate a test public key
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
		_, err = tox.AddFriendByPublicKey(keyPair.Public)
		b.StopTimer()

		if err != nil {
			b.Fatal(err)
		}

		tox.Kill()
	}
}

// BenchmarkSendFriendMessage measures message sending performance
func BenchmarkSendFriendMessage(b *testing.B) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	// Add a friend first
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	friendID, err := tox.AddFriendByPublicKey(keyPair.Public)
	if err != nil {
		b.Fatal(err)
	}

	message := "Benchmark test message"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tox.SendFriendMessage(friendID, message)
		// Note: This will fail in a real scenario due to friend not being online,
		// but we're measuring the API performance, not network performance
		_ = err
	}
}

// BenchmarkGetSavedata measures savedata serialization performance
func BenchmarkGetSavedata(b *testing.B) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	// Add some data to make the savedata more realistic
	err = tox.SelfSetName("Benchmark User")
	if err != nil {
		b.Fatal(err)
	}

	err = tox.SelfSetStatusMessage("Running benchmarks")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tox.GetSavedata()
	}
}

// BenchmarkSelfGetAddress measures ToxID generation performance
func BenchmarkSelfGetAddress(b *testing.B) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		b.Fatal(err)
	}
	defer tox.Kill()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tox.SelfGetAddress()
	}
}

// BenchmarkMultiNetworkIntegration performance benchmarks for integrated system
func BenchmarkMultiNetworkIntegration(b *testing.B) {
	addresses := []string{
		"192.168.1.1:33445",
		"[2001:db8::1]:33445",
		"test.onion:443",
		"test.b32.i2p:9150",
		"test.clients.nym:1789",
	}

	parser := transport.NewMultiNetworkParser()
	defer parser.Close()
	detector := transport.NewMultiNetworkDetector()

	b.Run("ParseDetectSelect", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := addresses[i%len(addresses)]

			// Parse
			addresses, _ := parser.Parse(addr)
			if len(addresses) > 0 {
				// Detect
				mockAddr := &mockAddr{network: "tcp", address: addr}
				_ = detector.DetectCapabilities(mockAddr)
			}
		}
	})

	b.Run("CrossNetworkCheck", func(b *testing.B) {
		testSources := []string{"192.168.1.1:33445", "example.onion:443"}
		testTargets := []string{"8.8.8.8:53", "target.onion:443"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			source := testSources[i%len(testSources)]
			target := testTargets[i%len(testTargets)]
			_ = checkNetworkCompatibility(source, target)
		}
	})
}
