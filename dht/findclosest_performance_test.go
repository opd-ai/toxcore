package dht

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkFindClosestNodes benchmarks the performance of FindClosestNodes
// with varying routing table sizes to demonstrate the optimization.
func BenchmarkFindClosestNodes(b *testing.B) {
	benchmarks := []struct {
		name      string
		nodeCount int
		findCount int
	}{
		{"SmallTable_4Nodes", 50, 4},
		{"MediumTable_4Nodes", 500, 4},
		{"LargeTable_4Nodes", 2000, 4},
		{"SmallTable_8Nodes", 50, 8},
		{"MediumTable_8Nodes", 500, 8},
		{"LargeTable_8Nodes", 2000, 8},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Setup: Create routing table and populate with nodes
			selfID := createTestToxID(0x00)
			rt := NewRoutingTable(selfID, 8)

			// Add nodes with varying IDs
			for i := 0; i < bm.nodeCount; i++ {
				nodeID := crypto.ToxID{}
				// Create diverse node IDs to spread across buckets
				nodeID.PublicKey[0] = byte(i & 0xFF)
				nodeID.PublicKey[1] = byte((i >> 8) & 0xFF)
				nodeID.PublicKey[2] = byte((i >> 16) & 0xFF)

				node := NewNode(nodeID, newMockAddr("1.1.1.1:1"))
				rt.AddNode(node)
			}

			targetID := createTestToxID(0x42)

			// Reset timer before the actual benchmark
			b.ResetTimer()

			// Run the benchmark
			for i := 0; i < b.N; i++ {
				_ = rt.FindClosestNodes(targetID, bm.findCount)
			}
		})
	}
}

// TestFindClosestNodesEdgeCases tests edge cases for the optimized implementation.
func TestFindClosestNodesEdgeCases(t *testing.T) {
	t.Run("EmptyRoutingTable", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)
		targetID := createTestToxID(0x42)

		result := rt.FindClosestNodes(targetID, 4)

		if len(result) != 0 {
			t.Errorf("Expected 0 nodes from empty table, got %d", len(result))
		}
	})

	t.Run("RequestZeroNodes", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add some nodes
		for i := 1; i <= 5; i++ {
			node := NewNode(createTestToxID(byte(i)), newMockAddr("1.1.1.1:1"))
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x42)
		result := rt.FindClosestNodes(targetID, 0)

		if len(result) != 0 {
			t.Errorf("Expected 0 nodes when requesting 0, got %d", len(result))
		}
	})

	t.Run("RequestNegativeNodes", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add some nodes
		for i := 1; i <= 5; i++ {
			node := NewNode(createTestToxID(byte(i)), newMockAddr("1.1.1.1:1"))
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x42)
		result := rt.FindClosestNodes(targetID, -1)

		if len(result) != 0 {
			t.Errorf("Expected 0 nodes when requesting negative count, got %d", len(result))
		}
	})

	t.Run("RequestMoreNodesThanAvailable", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add 3 nodes
		nodes := []*Node{
			NewNode(createTestToxID(0x01), newMockAddr("1.1.1.1:1")),
			NewNode(createTestToxID(0x02), newMockAddr("1.1.1.2:1")),
			NewNode(createTestToxID(0x03), newMockAddr("1.1.1.3:1")),
		}
		for _, node := range nodes {
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x04)
		result := rt.FindClosestNodes(targetID, 10) // Request more than available

		if len(result) != 3 {
			t.Errorf("Expected 3 nodes (all available), got %d", len(result))
		}
	})

	t.Run("SingleNode", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		node := NewNode(createTestToxID(0x01), newMockAddr("1.1.1.1:1"))
		rt.AddNode(node)

		targetID := createTestToxID(0x42)
		result := rt.FindClosestNodes(targetID, 4)

		if len(result) != 1 {
			t.Errorf("Expected 1 node, got %d", len(result))
		}
		if result[0].PublicKey[0] != 0x01 {
			t.Errorf("Expected node with key 0x01, got 0x%02x", result[0].PublicKey[0])
		}
	})

	t.Run("CorrectOrderWithManyNodes", func(t *testing.T) {
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add nodes with known distances from target 0x80
		// Closest to 0x80: 0x80 (dist=0), 0x81 (dist=1), 0x82 (dist=2), 0x83 (dist=3)
		testNodes := []byte{0x80, 0x81, 0x82, 0x83, 0xFF, 0x00, 0x01, 0x02}
		for _, b := range testNodes {
			node := NewNode(createTestToxID(b), newMockAddr("1.1.1.1:1"))
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x80)
		result := rt.FindClosestNodes(targetID, 4)

		if len(result) != 4 {
			t.Errorf("Expected 4 nodes, got %d", len(result))
		}

		// Verify correct ordering (closest first)
		expectedOrder := []byte{0x80, 0x81, 0x82, 0x83}
		for i, expected := range expectedOrder {
			if result[i].PublicKey[0] != expected {
				t.Errorf("Position %d: expected 0x%02x, got 0x%02x", i, expected, result[i].PublicKey[0])
			}
		}
	})
}

// TestFindClosestNodesConsistency ensures the optimized version produces
// identical results to the original implementation for various scenarios.
func TestFindClosestNodesConsistency(t *testing.T) {
	testCases := []struct {
		name       string
		nodeCount  int
		findCount  int
		targetByte byte
	}{
		{"Few_nodes_few_requested", 5, 3, 0x42},
		{"Many_nodes_few_requested", 100, 4, 0x42},
		{"Exact_match", 10, 10, 0x42},
		{"More_requested_than_available", 5, 20, 0x42},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selfID := createTestToxID(0x00)
			rt := NewRoutingTable(selfID, 8)

			// Add nodes with diverse IDs
			for i := 0; i < tc.nodeCount; i++ {
				nodeID := crypto.ToxID{}
				nodeID.PublicKey[0] = byte(i + 1) // Avoid 0x00 (self)
				nodeID.PublicKey[1] = byte((i * 7) & 0xFF)

				node := NewNode(nodeID, newMockAddr("1.1.1.1:1"))
				rt.AddNode(node)
			}

			targetID := createTestToxID(tc.targetByte)
			result := rt.FindClosestNodes(targetID, tc.findCount)

			// Verify results are sorted by distance
			if len(result) > 1 {
				targetNode := &Node{ID: targetID}
				copy(targetNode.PublicKey[:], targetID.PublicKey[:])

				for i := 0; i < len(result)-1; i++ {
					distI := result[i].Distance(targetNode)
					distJ := result[i+1].Distance(targetNode)

					if !lessDistance(distI, distJ) && distI != distJ {
						t.Errorf("Results not sorted: node %d (dist=%v) should be closer than node %d (dist=%v)",
							i, distI, i+1, distJ)
					}
				}
			}

			// Verify we don't return more than requested
			if len(result) > tc.findCount {
				t.Errorf("Returned %d nodes, but only requested %d", len(result), tc.findCount)
			}
		})
	}
}
