package dht

import (
	"fmt"
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkNewNode measures node creation performance
func BenchmarkNewNode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}

		nospam, err := crypto.GenerateNospam()
		if err != nil {
			b.Fatal(err)
		}

		toxID := crypto.NewToxID(keyPair.Public, nospam)

		addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
		if err != nil {
			b.Fatal(err)
		}

		_ = NewNode(*toxID, addr)
	}
}

// BenchmarkKBucketAddNode measures k-bucket node addition performance
func BenchmarkKBucketAddNode(b *testing.B) {
	bucket := NewKBucket(20) // Standard k-bucket size

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}

		nospam, err := crypto.GenerateNospam()
		if err != nil {
			b.Fatal(err)
		}

		toxID := crypto.NewToxID(keyPair.Public, nospam)

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 12000+i%1000))
		if err != nil {
			b.Fatal(err)
		}

		node := NewNode(*toxID, addr)
		bucket.AddNode(node)
	}
}

// BenchmarkKBucketGetNodes measures k-bucket node retrieval performance
func BenchmarkKBucketGetNodes(b *testing.B) {
	bucket := NewKBucket(20) // Standard k-bucket size

	// Pre-populate the bucket with some nodes
	for i := 0; i < 10; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}

		nospam, err := crypto.GenerateNospam()
		if err != nil {
			b.Fatal(err)
		}

		toxID := crypto.NewToxID(keyPair.Public, nospam)

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 12000+i))
		if err != nil {
			b.Fatal(err)
		}

		node := NewNode(*toxID, addr)
		bucket.AddNode(node)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bucket.GetNodes()
	}
}
