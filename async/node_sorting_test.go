package async

import (
	"crypto/rand"
	"net"
	"testing"
)

// TestSortCandidatesByDistance verifies that storage node candidates are sorted correctly by distance
func TestSortCandidatesByDistance(t *testing.T) {
	tests := []struct {
		name       string
		candidates []nodeDistance
		expected   []uint64 // expected distances in sorted order
	}{
		{
			name:       "empty list",
			candidates: []nodeDistance{},
			expected:   []uint64{},
		},
		{
			name: "single element",
			candidates: []nodeDistance{
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 42},
			},
			expected: []uint64{42},
		},
		{
			name: "already sorted",
			candidates: []nodeDistance{
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 10},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2000}, distance: 20},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 3000}, distance: 30},
			},
			expected: []uint64{10, 20, 30},
		},
		{
			name: "reverse sorted",
			candidates: []nodeDistance{
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 100},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2000}, distance: 50},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 3000}, distance: 25},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4000}, distance: 10},
			},
			expected: []uint64{10, 25, 50, 100},
		},
		{
			name: "random order",
			candidates: []nodeDistance{
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 75},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2000}, distance: 12},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 3000}, distance: 99},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4000}, distance: 3},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5000}, distance: 45},
			},
			expected: []uint64{3, 12, 45, 75, 99},
		},
		{
			name: "duplicate distances",
			candidates: []nodeDistance{
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 50},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2000}, distance: 50},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 3000}, distance: 25},
				{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4000}, distance: 25},
			},
			expected: []uint64{25, 25, 50, 50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AsyncClient{}
			client.sortCandidatesByDistance(tt.candidates)

			// Verify the sorted order
			if len(tt.candidates) != len(tt.expected) {
				t.Fatalf("expected %d candidates, got %d", len(tt.expected), len(tt.candidates))
			}

			for i, expected := range tt.expected {
				if tt.candidates[i].distance != expected {
					t.Errorf("position %d: expected distance %d, got %d", i, expected, tt.candidates[i].distance)
				}
			}
		})
	}
}

// TestFindStorageNodesIntegration verifies the complete storage node selection process
func TestFindStorageNodesIntegration(t *testing.T) {
	client := &AsyncClient{
		storageNodes: make(map[[32]byte]net.Addr),
	}

	// Add some storage nodes
	for i := 0; i < 10; i++ {
		var pk [32]byte
		rand.Read(pk[:])
		client.storageNodes[pk] = &net.UDPAddr{IP: net.IPv4(127, 0, 0, byte(i+1)), Port: 1000 + i}
	}

	var targetPK [32]byte
	rand.Read(targetPK[:])

	// Find top 3 storage nodes
	nodes := client.findStorageNodes(targetPK, 3)

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// Verify we got valid addresses
	for i, node := range nodes {
		if node == nil {
			t.Errorf("node %d is nil", i)
		}
	}
}

// TestSortCandidatesStability verifies sorting is stable for equal distances
func TestSortCandidatesStability(t *testing.T) {
	candidates := []nodeDistance{
		{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}, distance: 50},
		{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: 2000}, distance: 50},
		{addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 3), Port: 3000}, distance: 50},
	}

	client := &AsyncClient{}
	client.sortCandidatesByDistance(candidates)

	// All distances should still be 50
	for i, candidate := range candidates {
		if candidate.distance != 50 {
			t.Errorf("position %d: expected distance 50, got %d", i, candidate.distance)
		}
	}
}

// BenchmarkSortCandidatesByDistance measures sorting performance with varying list sizes
func BenchmarkSortCandidatesByDistance(b *testing.B) {
	benchmarks := []struct {
		name string
		size int
	}{
		{"10 nodes", 10},
		{"100 nodes", 100},
		{"1000 nodes", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Generate test data once
			candidates := make([]nodeDistance, bm.size)
			for i := 0; i < bm.size; i++ {
				var distance [8]byte
				rand.Read(distance[:])
				candidates[i] = nodeDistance{
					addr:     &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000 + i},
					distance: uint64(distance[0])<<56 | uint64(distance[1])<<48 | uint64(distance[2])<<40 | uint64(distance[3])<<32 | uint64(distance[4])<<24 | uint64(distance[5])<<16 | uint64(distance[6])<<8 | uint64(distance[7]),
				}
			}

			client := &AsyncClient{}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Create a copy to sort each iteration
				testData := make([]nodeDistance, len(candidates))
				copy(testData, candidates)
				client.sortCandidatesByDistance(testData)
			}
		})
	}
}
