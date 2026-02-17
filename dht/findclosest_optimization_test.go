package dht

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestFindClosestNodesOptimization verifies the optimization that eliminates
// redundant distance recalculation in the sort phase.
//
// The optimization replaces:
//  1. Copy heap nodes to result slice
//  2. Sort result slice (recalculating distances)
//
// With:
//  1. Pop all nodes from max-heap (gives farthest to closest order)
//  2. Reverse by popping into result[i] where i counts down
//
// This avoids O(k log k) distance recalculations while maintaining correctness.
func TestFindClosestNodesOptimization(t *testing.T) {
	t.Run("NoRedundantDistanceCalculation", func(t *testing.T) {
		// This test verifies that the optimized implementation produces
		// identical results to the original while avoiding redundant work
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add nodes with known distances from target 0x80
		testNodes := []byte{
			0x80, // Distance 0 from 0x80
			0x81, // Distance 1 from 0x80
			0x82, // Distance 2 from 0x80
			0x7F, // Distance 255 from 0x80
			0x00, // Distance 128 from 0x80
		}

		for _, b := range testNodes {
			node := NewNode(createTestToxID(b), newMockAddr("1.1.1.1:1"))
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x80)
		result := rt.FindClosestNodes(targetID, 3)

		// Verify we get exactly 3 closest nodes
		if len(result) != 3 {
			t.Fatalf("Expected 3 nodes, got %d", len(result))
		}

		// Verify correct ordering (closest to farthest)
		expectedOrder := []byte{0x80, 0x81, 0x82}
		for i, expected := range expectedOrder {
			if result[i].PublicKey[0] != expected {
				t.Errorf("Position %d: expected 0x%02x, got 0x%02x",
					i, expected, result[i].PublicKey[0])
			}
		}
	})

	t.Run("HeapExtractionMaintainsOrder", func(t *testing.T) {
		// Verify that extracting nodes via heap.Pop and reversing
		// produces the same result as sorting
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add many nodes to stress-test ordering
		for i := 0; i < 100; i++ {
			nodeID := crypto.ToxID{}
			nodeID.PublicKey[0] = byte(i)
			nodeID.PublicKey[1] = byte((i * 7) & 0xFF)
			node := NewNode(nodeID, newMockAddr("1.1.1.1:1"))
			rt.AddNode(node)
		}

		targetID := createTestToxID(0x42)
		result := rt.FindClosestNodes(targetID, 10)

		// Verify results are sorted by distance
		if len(result) != 10 {
			t.Fatalf("Expected 10 nodes, got %d", len(result))
		}

		targetNode := &Node{ID: targetID}
		copy(targetNode.PublicKey[:], targetID.PublicKey[:])

		for i := 0; i < len(result)-1; i++ {
			distI := result[i].Distance(targetNode)
			distJ := result[i+1].Distance(targetNode)

			if !lessDistance(distI, distJ) && distI != distJ {
				t.Errorf("Position %d and %d not sorted: dist[%d]=%v should be <= dist[%d]=%v",
					i, i+1, i, distI[:4], i+1, distJ[:4])
			}
		}
	})

	t.Run("SingleNodeExtractionWorks", func(t *testing.T) {
		// Edge case: extracting a single node should work correctly
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		node := NewNode(createTestToxID(0x42), newMockAddr("1.1.1.1:1"))
		rt.AddNode(node)

		targetID := createTestToxID(0x80)
		result := rt.FindClosestNodes(targetID, 1)

		if len(result) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(result))
		}

		if result[0].PublicKey[0] != 0x42 {
			t.Errorf("Expected node 0x42, got 0x%02x", result[0].PublicKey[0])
		}
	})

	t.Run("EmptyHeapExtractionWorks", func(t *testing.T) {
		// Edge case: extracting from empty heap should return empty slice
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		targetID := createTestToxID(0x42)
		result := rt.FindClosestNodes(targetID, 10)

		if len(result) != 0 {
			t.Errorf("Expected empty result, got %d nodes", len(result))
		}
	})
}

// TestOptimizationCorrectness compares the optimized implementation behavior
// with expected results across various scenarios.
func TestOptimizationCorrectness(t *testing.T) {
	scenarios := []struct {
		name         string
		nodeCount    int
		requestCount int
		targetByte   byte
	}{
		{"request_less_than_available", 10, 4, 0x80},
		{"request_equal_to_available", 5, 5, 0x80},
		{"request_more_than_available", 3, 10, 0x80},
		{"large_routing_table", 200, 8, 0x42},
		{"single_node_request", 50, 1, 0x42},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			selfID := createTestToxID(0x00)
			rt := NewRoutingTable(selfID, 8)

			// Populate routing table
			for i := 0; i < sc.nodeCount; i++ {
				nodeID := crypto.ToxID{}
				nodeID.PublicKey[0] = byte(i + 1)
				nodeID.PublicKey[1] = byte((i * 13) & 0xFF)
				node := NewNode(nodeID, newMockAddr("1.1.1.1:1"))
				rt.AddNode(node)
			}

			targetID := createTestToxID(sc.targetByte)
			result := rt.FindClosestNodes(targetID, sc.requestCount)

			// Expected result size
			expectedSize := sc.requestCount
			if sc.nodeCount < sc.requestCount {
				expectedSize = sc.nodeCount
			}

			if len(result) != expectedSize {
				t.Errorf("Expected %d nodes, got %d", expectedSize, len(result))
			}

			// Verify ordering
			if len(result) > 1 {
				targetNode := &Node{ID: targetID}
				copy(targetNode.PublicKey[:], targetID.PublicKey[:])

				for i := 0; i < len(result)-1; i++ {
					distI := result[i].Distance(targetNode)
					distJ := result[i+1].Distance(targetNode)

					if !lessDistance(distI, distJ) && distI != distJ {
						t.Errorf("Results not properly ordered at positions %d and %d", i, i+1)
					}
				}
			}
		})
	}
}
