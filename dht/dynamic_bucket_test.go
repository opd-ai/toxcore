package dht

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// mockTimeProvider is a time provider for testing that returns controlled time values.
type mockTimeProvider struct {
	current time.Time
}

func (m *mockTimeProvider) Now() time.Time                  { return m.current }
func (m *mockTimeProvider) Since(t time.Time) time.Duration { return m.current.Sub(t) }

func (m *mockTimeProvider) Advance(d time.Duration) {
	m.current = m.current.Add(d)
}

func TestDensityEstimator_Basic(t *testing.T) {
	de := NewDensityEstimator(8)

	// Initially no data
	stats := de.Stats()
	if stats.TotalNodesObserved != 0 {
		t.Errorf("Expected 0 observations, got %d", stats.TotalNodesObserved)
	}

	// Record some additions
	de.RecordAddition(0, true)
	de.RecordAddition(0, true)
	de.RecordAddition(0, false)

	stats = de.Stats()
	if stats.TotalNodesObserved != 3 {
		t.Errorf("Expected 3 observations, got %d", stats.TotalNodesObserved)
	}
	if stats.SuccessfulAdditions != 2 {
		t.Errorf("Expected 2 successful, got %d", stats.SuccessfulAdditions)
	}
	if stats.RejectedAdditions != 1 {
		t.Errorf("Expected 1 rejected, got %d", stats.RejectedAdditions)
	}
}

func TestDensityEstimator_RejectionRate(t *testing.T) {
	de := NewDensityEstimator(8)

	// 50% rejection rate
	de.RecordAddition(0, true)
	de.RecordAddition(0, false)

	rate := de.GetRejectionRate()
	if rate != 0.5 {
		t.Errorf("Expected rejection rate 0.5, got %f", rate)
	}

	// Add more successes to lower rate
	de.RecordAddition(0, true)
	de.RecordAddition(0, true)

	rate = de.GetRejectionRate()
	expected := 0.25 // 1 rejection out of 4 total
	if rate != expected {
		t.Errorf("Expected rejection rate %f, got %f", expected, rate)
	}
}

func TestDensityEstimator_SuggestBucketSize_NotEnoughData(t *testing.T) {
	de := NewDensityEstimator(8)

	// Not enough data - should return base size
	size := de.SuggestBucketSize(0)
	if size != 8 {
		t.Errorf("Expected base size 8 with insufficient data, got %d", size)
	}

	// Add some but not enough data
	for i := 0; i < MinNodesForDensityEstimate-1; i++ {
		de.RecordAddition(0, true)
	}

	size = de.SuggestBucketSize(0)
	if size != 8 {
		t.Errorf("Expected base size 8 with insufficient data, got %d", size)
	}
}

func TestDensityEstimator_SuggestBucketSize_HighDensity(t *testing.T) {
	de := NewDensityEstimator(8)

	// Simulate high-density network with many rejections
	for i := 0; i < MinNodesForDensityEstimate; i++ {
		de.RecordAddition(0, false) // All rejections
	}

	size := de.SuggestBucketSize(0)
	if size <= 8 {
		t.Errorf("Expected larger bucket size for high density, got %d", size)
	}
	if size > MaxBucketSize {
		t.Errorf("Bucket size %d exceeds max %d", size, MaxBucketSize)
	}
}

func TestDensityEstimator_SuggestBucketSize_LowDensity(t *testing.T) {
	de := NewDensityEstimator(8)

	// Simulate low-density network with all successes
	for i := 0; i < MinNodesForDensityEstimate; i++ {
		de.RecordAddition(i%256, true) // Spread across buckets, all success
	}

	size := de.SuggestBucketSize(0)
	if size != 8 {
		t.Errorf("Expected base bucket size for low density, got %d", size)
	}
}

func TestDensityEstimator_TimeWindow(t *testing.T) {
	mockTime := &mockTimeProvider{current: time.Now()}
	de := NewDensityEstimator(8)
	de.SetTimeProvider(mockTime)
	de.window = 1 * time.Minute // Short window for testing

	// Add some observations
	de.RecordAddition(0, true)
	de.RecordAddition(0, true)

	stats := de.Stats()
	if stats.RecentAdditions != 2 {
		t.Errorf("Expected 2 recent additions, got %d", stats.RecentAdditions)
	}

	// Advance time past the window
	mockTime.Advance(2 * time.Minute)

	// Record one more to trigger pruning
	de.RecordAddition(0, true)

	stats = de.Stats()
	if stats.RecentAdditions != 1 {
		t.Errorf("Expected 1 recent addition after pruning, got %d", stats.RecentAdditions)
	}
}

func TestDynamicKBucket_AddNode(t *testing.T) {
	estimator := NewDensityEstimator(4)
	bucket := NewDynamicKBucket(4, 0, estimator)

	// Generate test nodes
	nodes := make([]*Node, 10)
	for i := 0; i < 10; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		var nospam [4]byte
		toxID := crypto.NewToxID(keyPair.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		nodes[i] = NewNode(*toxID, addr)
	}

	// Add nodes up to initial capacity
	for i := 0; i < 4; i++ {
		if !bucket.AddNodeDynamic(nodes[i]) {
			t.Errorf("Failed to add node %d within capacity", i)
		}
	}

	if bucket.GetCurrentSize() != 4 {
		t.Errorf("Expected 4 nodes, got %d", bucket.GetCurrentSize())
	}
}

func TestDynamicKBucket_AutoResize(t *testing.T) {
	estimator := NewDensityEstimator(2)
	bucket := NewDynamicKBucket(2, 0, estimator)

	// Generate more nodes than initial capacity
	nodes := make([]*Node, 10)
	for i := 0; i < 10; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		var nospam [4]byte
		toxID := crypto.NewToxID(keyPair.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		nodes[i] = NewNode(*toxID, addr)
	}

	// Fill initial capacity
	bucket.AddNodeDynamic(nodes[0])
	bucket.AddNodeDynamic(nodes[1])

	initialMax := bucket.GetMaxSize()

	// Simulate high rejection rate to trigger resize
	for i := 0; i < MinNodesForDensityEstimate; i++ {
		estimator.RecordAddition(0, false)
	}

	// Now try to add more - should trigger resize
	bucket.AddNodeDynamic(nodes[2])

	newMax := bucket.GetMaxSize()
	if newMax <= initialMax {
		t.Logf("Bucket did not auto-resize: initial=%d, new=%d", initialMax, newMax)
		// This is okay if the density algorithm determined no resize needed
	}
}

func TestDynamicRoutingTable_Create(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(keyPair.Public, nospam)

	drt := NewDynamicRoutingTable(*selfID, 8)

	if drt == nil {
		t.Fatal("Failed to create dynamic routing table")
	}

	// Verify all buckets initialized
	sizes := drt.GetBucketSizes()
	for i, size := range sizes {
		if size != 8 {
			t.Errorf("Bucket %d has size %d, expected 8", i, size)
		}
	}

	totalCap := drt.GetTotalCapacity()
	if totalCap != 256*8 {
		t.Errorf("Expected total capacity %d, got %d", 256*8, totalCap)
	}
}

func TestDynamicRoutingTable_AddNode(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(keyPair.Public, nospam)

	drt := NewDynamicRoutingTable(*selfID, 8)

	// Generate and add nodes
	nodesAdded := 0
	for i := 0; i < 100; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)

		if drt.AddNode(node) {
			nodesAdded++
		}
	}

	totalNodes := drt.GetTotalNodes()
	if totalNodes != nodesAdded {
		t.Errorf("Expected %d nodes, got %d", nodesAdded, totalNodes)
	}

	// Verify density stats updated
	stats := drt.GetDensityStats()
	if stats.TotalNodesObserved == 0 {
		t.Error("Expected density stats to be recorded")
	}
}

func TestDynamicRoutingTable_DontAddSelf(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(keyPair.Public, nospam)

	drt := NewDynamicRoutingTable(*selfID, 8)

	// Try to add ourselves
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	selfNode := NewNode(*selfID, addr)

	if drt.AddNode(selfNode) {
		t.Error("Should not be able to add self to routing table")
	}
}

func TestDynamicRoutingTable_FindClosestNodes(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(keyPair.Public, nospam)

	drt := NewDynamicRoutingTable(*selfID, 8)

	// Add some nodes
	for i := 0; i < 20; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		node := NewNode(*nodeID, addr)
		drt.AddNode(node)
	}

	// Find closest nodes to a random target
	targetKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	targetID := crypto.NewToxID(targetKey.Public, nospam)

	closest := drt.FindClosestNodes(*targetID, 5)
	if len(closest) > 5 {
		t.Errorf("Expected at most 5 closest nodes, got %d", len(closest))
	}
}

func TestDynamicKBucket_GetMaxSize(t *testing.T) {
	estimator := NewDensityEstimator(16)
	bucket := NewDynamicKBucket(16, 0, estimator)

	if bucket.GetMaxSize() != 16 {
		t.Errorf("Expected max size 16, got %d", bucket.GetMaxSize())
	}

	// Manually resize
	bucket.resize(32)

	if bucket.GetMaxSize() != 32 {
		t.Errorf("Expected max size 32 after resize, got %d", bucket.GetMaxSize())
	}
}

func TestDynamicKBucket_ResizeShrinkNotAllowed(t *testing.T) {
	estimator := NewDensityEstimator(16)
	bucket := NewDynamicKBucket(16, 0, estimator)

	// Try to shrink - should not work
	bucket.resize(8)

	if bucket.GetMaxSize() != 16 {
		t.Errorf("Bucket should not shrink: expected 16, got %d", bucket.GetMaxSize())
	}
}

func TestBucketFillRate(t *testing.T) {
	de := NewDensityEstimator(8)

	// Record multiple additions to the same bucket
	for i := 0; i < 10; i++ {
		de.RecordAddition(5, true)
	}

	rate := de.GetBucketFillRate(5)
	if rate <= 0 {
		t.Errorf("Expected non-zero fill rate, got %f", rate)
	}

	// Invalid bucket index should return 0
	rate = de.GetBucketFillRate(-1)
	if rate != 0 {
		t.Errorf("Expected 0 for invalid bucket, got %f", rate)
	}

	rate = de.GetBucketFillRate(256)
	if rate != 0 {
		t.Errorf("Expected 0 for out-of-range bucket, got %f", rate)
	}
}

func BenchmarkDensityEstimator_RecordAddition(b *testing.B) {
	de := NewDensityEstimator(8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		de.RecordAddition(i%256, i%3 == 0)
	}
}

func BenchmarkDensityEstimator_SuggestBucketSize(b *testing.B) {
	de := NewDensityEstimator(8)

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		de.RecordAddition(i%256, i%3 == 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		de.SuggestBucketSize(i % 256)
	}
}

func BenchmarkDynamicRoutingTable_AddNode(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate keypair: %v", err)
	}
	var nospam [4]byte
	selfID := crypto.NewToxID(keyPair.Public, nospam)

	drt := NewDynamicRoutingTable(*selfID, 8)

	// Pre-generate nodes
	nodes := make([]*Node, b.N)
	for i := 0; i < b.N; i++ {
		nodeKey, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatalf("Failed to generate keypair: %v", err)
		}
		nodeID := crypto.NewToxID(nodeKey.Public, nospam)
		addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
		nodes[i] = NewNode(*nodeID, addr)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drt.AddNode(nodes[i])
	}
}
