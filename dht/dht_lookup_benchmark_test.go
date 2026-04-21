package dht

import (
	"fmt"
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// populateRoutingTable fills a routing table with n random nodes.
// It is a helper shared by the DHT latency benchmarks.
func populateRoutingTable(b *testing.B, rt *RoutingTable, n int) {
	b.Helper()
	var addr net.Addr = &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
	for i := 0; i < n; i++ {
		kp, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatalf("GenerateKeyPair: %v", err)
		}
		var nospam [4]byte
		nospam[0] = byte(i)
		id := crypto.NewToxID(kp.Public, nospam)
		rt.AddNode(NewNode(*id, addr))
	}
}

// makeTargetID constructs a deterministic ToxID for benchmarks.
func makeTargetID(b *testing.B, seed byte) crypto.ToxID {
	b.Helper()
	var pub [32]byte
	for i := range pub {
		pub[i] = seed + byte(i)
	}
	var nospam [4]byte
	id := crypto.NewToxID(pub, nospam)
	return *id
}

// BenchmarkFindClosestNodesByTableSize measures FindClosestNodes latency at
// various routing-table sizes, simulating the typical DHT lookup hot path.
func BenchmarkFindClosestNodesByTableSize(b *testing.B) {
	tableSizes := []int{10, 100, 500, 2000}
	const k = 8 // standard Tox/Kademlia k value

	for _, size := range tableSizes {
		b.Run(fmt.Sprintf("table_%d", size), func(b *testing.B) {
			selfKP, err := crypto.GenerateKeyPair()
			if err != nil {
				b.Fatal(err)
			}
			var selfNospam [4]byte
			selfID := crypto.NewToxID(selfKP.Public, selfNospam)

			rt := NewRoutingTable(*selfID, 20)
			populateRoutingTable(b, rt, size)

			target := makeTargetID(b, 0xab)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				nodes := rt.FindClosestNodes(target, k)
				_ = nodes
			}
		})
	}
}

// BenchmarkFindClosestNodesNoCache is the same as BenchmarkFindClosestNodes
// but bypasses the lookup cache to measure raw tree traversal cost.
func BenchmarkFindClosestNodesNoCache(b *testing.B) {
	tableSizes := []int{10, 100, 500, 2000}
	const k = 8

	for _, size := range tableSizes {
		b.Run(fmt.Sprintf("table_%d", size), func(b *testing.B) {
			selfKP, err := crypto.GenerateKeyPair()
			if err != nil {
				b.Fatal(err)
			}
			var selfNospam [4]byte
			selfID := crypto.NewToxID(selfKP.Public, selfNospam)

			rt := NewRoutingTable(*selfID, 20)
			populateRoutingTable(b, rt, size)

			target := makeTargetID(b, 0xcd)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				nodes := rt.FindClosestNodesNoCache(target, k)
				_ = nodes
			}
		})
	}
}

// BenchmarkRoutingTableAddNode measures the cost of inserting nodes into a
// routing table as it grows, tracking how latency changes with table size.
func BenchmarkRoutingTableAddNode(b *testing.B) {
	selfKP, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	var selfNospam [4]byte
	selfID := crypto.NewToxID(selfKP.Public, selfNospam)

	rt := NewRoutingTable(*selfID, 20)
	var addr net.Addr = &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kp, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatalf("GenerateKeyPair: %v", err)
		}
		var nospam [4]byte
		nospam[0] = byte(i)
		id := crypto.NewToxID(kp.Public, nospam)
		rt.AddNode(NewNode(*id, addr))
	}
}

// BenchmarkLookupCacheHit measures the cache-hit path of FindClosestNodes.
func BenchmarkLookupCacheHit(b *testing.B) {
	selfKP, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	var selfNospam [4]byte
	selfID := crypto.NewToxID(selfKP.Public, selfNospam)

	rt := NewRoutingTable(*selfID, 20)
	populateRoutingTable(b, rt, 500)

	target := makeTargetID(b, 0xef)

	// Warm the cache with one call.
	_ = rt.FindClosestNodes(target, 8)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nodes := rt.FindClosestNodes(target, 8)
		_ = nodes
	}
}
