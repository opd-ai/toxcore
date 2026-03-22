package dht

import (
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

const (
	// DefaultBaseBucketSize is the minimum k-bucket size.
	DefaultBaseBucketSize = 8

	// MaxBucketSize is the maximum k-bucket size to prevent unbounded growth.
	MaxBucketSize = 64

	// DensityEstimationWindow is how long to consider node additions when estimating density.
	DensityEstimationWindow = 5 * time.Minute

	// DensityHighThreshold is the minimum density factor for bucket expansion.
	// When observed density exceeds this multiplier of base capacity, buckets expand.
	DensityHighThreshold = 0.9

	// DensityLowThreshold is the maximum density factor for bucket contraction.
	// When observed density falls below this multiplier, buckets may contract.
	DensityLowThreshold = 0.3

	// MinNodesForDensityEstimate is the minimum nodes needed before adjusting bucket sizes.
	MinNodesForDensityEstimate = 20
)

// DensityEstimator tracks network density to inform bucket sizing decisions.
// It observes node addition patterns over time to estimate network population.
type DensityEstimator struct {
	mu sync.RWMutex

	// timestamps of recent node additions for rate estimation
	additionTimes []time.Time

	// observed bucket fill rates per bucket index (0-255)
	bucketFillRates [256]float64

	// total nodes observed (including additions and removals)
	totalNodesObserved uint64

	// successful additions vs rejections (bucket full)
	successfulAdditions uint64
	rejectedAdditions   uint64

	// configuration
	window       time.Duration
	baseBucket   int
	timeProvider TimeProvider
}

// NewDensityEstimator creates a new density estimator with default settings.
func NewDensityEstimator(baseBucketSize int) *DensityEstimator {
	if baseBucketSize <= 0 {
		baseBucketSize = DefaultBaseBucketSize
	}
	return &DensityEstimator{
		additionTimes: make([]time.Time, 0, 1000),
		window:        DensityEstimationWindow,
		baseBucket:    baseBucketSize,
		timeProvider:  nil,
	}
}

// SetTimeProvider sets a custom time provider for testing.
func (de *DensityEstimator) SetTimeProvider(tp TimeProvider) {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.timeProvider = tp
}

// getTime returns current time from the configured provider.
func (de *DensityEstimator) getTime() time.Time {
	if de.timeProvider != nil {
		return de.timeProvider.Now()
	}
	return time.Now()
}

// RecordAddition records a node addition attempt and its result.
func (de *DensityEstimator) RecordAddition(bucketIndex int, success bool) {
	de.mu.Lock()
	defer de.mu.Unlock()

	now := de.getTime()
	de.additionTimes = append(de.additionTimes, now)
	de.totalNodesObserved++

	if success {
		de.successfulAdditions++
	} else {
		de.rejectedAdditions++
	}

	// Update bucket fill rate (exponential moving average)
	if bucketIndex >= 0 && bucketIndex < 256 {
		rate := &de.bucketFillRates[bucketIndex]
		if success {
			*rate = *rate*0.9 + 0.1 // Decay toward 1 on success
		} else {
			*rate = *rate*0.9 + 1.0*0.1 // Move toward 1 on rejection (bucket was full)
		}
	}

	// Prune old timestamps outside the window
	de.pruneOldTimestamps(now)
}

// pruneOldTimestamps removes timestamps older than the estimation window.
func (de *DensityEstimator) pruneOldTimestamps(now time.Time) {
	cutoff := now.Add(-de.window)
	newTimes := make([]time.Time, 0, len(de.additionTimes))
	for _, t := range de.additionTimes {
		if t.After(cutoff) {
			newTimes = append(newTimes, t)
		}
	}
	de.additionTimes = newTimes
}

// GetAdditionRate returns the rate of node additions per minute within the window.
func (de *DensityEstimator) GetAdditionRate() float64 {
	de.mu.RLock()
	defer de.mu.RUnlock()

	if len(de.additionTimes) < 2 {
		return 0
	}

	now := de.getTime()
	de.mu.RUnlock()
	de.mu.Lock()
	de.pruneOldTimestamps(now)
	count := len(de.additionTimes)
	de.mu.Unlock()
	de.mu.RLock()

	if count < 2 {
		return 0
	}

	// Calculate rate as additions per minute
	windowMinutes := de.window.Minutes()
	if windowMinutes <= 0 {
		windowMinutes = 1
	}
	return float64(count) / windowMinutes
}

// GetRejectionRate returns the fraction of additions that were rejected.
func (de *DensityEstimator) GetRejectionRate() float64 {
	de.mu.RLock()
	defer de.mu.RUnlock()

	total := de.successfulAdditions + de.rejectedAdditions
	if total == 0 {
		return 0
	}
	return float64(de.rejectedAdditions) / float64(total)
}

// GetBucketFillRate returns the estimated fill rate for a specific bucket.
func (de *DensityEstimator) GetBucketFillRate(bucketIndex int) float64 {
	de.mu.RLock()
	defer de.mu.RUnlock()

	if bucketIndex < 0 || bucketIndex >= 256 {
		return 0
	}
	return de.bucketFillRates[bucketIndex]
}

// SuggestBucketSize returns a recommended bucket size based on observed density.
// Returns a size between baseBucketSize and MaxBucketSize.
func (de *DensityEstimator) SuggestBucketSize(bucketIndex int) int {
	de.mu.RLock()
	defer de.mu.RUnlock()

	// Need enough data to make estimates
	if de.totalNodesObserved < MinNodesForDensityEstimate {
		return de.baseBucket
	}

	rejectionRate := float64(0)
	if de.successfulAdditions+de.rejectedAdditions > 0 {
		rejectionRate = float64(de.rejectedAdditions) / float64(de.successfulAdditions+de.rejectedAdditions)
	}

	bucketFillRate := float64(0)
	if bucketIndex >= 0 && bucketIndex < 256 {
		bucketFillRate = de.bucketFillRates[bucketIndex]
	}

	// Calculate suggested size based on rejection rate and bucket fill
	// High rejection rate + high fill rate = need bigger buckets
	densityFactor := (rejectionRate + bucketFillRate) / 2

	var suggestedSize int
	if densityFactor > DensityHighThreshold {
		// Network is dense, expand buckets
		expansion := 1.0 + (densityFactor-DensityHighThreshold)*4 // Up to 4x expansion
		suggestedSize = int(float64(de.baseBucket) * expansion)
	} else if densityFactor < DensityLowThreshold {
		// Network is sparse, keep at base size
		suggestedSize = de.baseBucket
	} else {
		// Proportional scaling between thresholds
		scaleFactor := (densityFactor - DensityLowThreshold) / (DensityHighThreshold - DensityLowThreshold)
		suggestedSize = de.baseBucket + int(float64(MaxBucketSize-de.baseBucket)*scaleFactor*0.5)
	}

	// Clamp to valid range
	if suggestedSize < de.baseBucket {
		suggestedSize = de.baseBucket
	}
	if suggestedSize > MaxBucketSize {
		suggestedSize = MaxBucketSize
	}

	return suggestedSize
}

// Stats returns density estimation statistics.
func (de *DensityEstimator) Stats() DensityStats {
	de.mu.RLock()
	defer de.mu.RUnlock()

	return DensityStats{
		TotalNodesObserved:  de.totalNodesObserved,
		SuccessfulAdditions: de.successfulAdditions,
		RejectedAdditions:   de.rejectedAdditions,
		RecentAdditions:     len(de.additionTimes),
		BaseBucketSize:      de.baseBucket,
	}
}

// DensityStats contains density estimation statistics.
type DensityStats struct {
	TotalNodesObserved  uint64
	SuccessfulAdditions uint64
	RejectedAdditions   uint64
	RecentAdditions     int
	BaseBucketSize      int
}

// DynamicKBucket extends KBucket with dynamic size adjustment.
type DynamicKBucket struct {
	KBucket
	densityEstimator *DensityEstimator
	bucketIndex      int
}

// NewDynamicKBucket creates a k-bucket with dynamic sizing capability.
func NewDynamicKBucket(initialSize, bucketIndex int, estimator *DensityEstimator) *DynamicKBucket {
	return &DynamicKBucket{
		KBucket: KBucket{
			nodes:   make([]*Node, 0, initialSize),
			maxSize: initialSize,
		},
		densityEstimator: estimator,
		bucketIndex:      bucketIndex,
	}
}

// AddNodeDynamic adds a node and updates density estimates, potentially resizing.
func (dkb *DynamicKBucket) AddNodeDynamic(node *Node) bool {
	// Try to add with current size
	success := dkb.KBucket.AddNode(node)

	// Record the result for density estimation
	if dkb.densityEstimator != nil {
		dkb.densityEstimator.RecordAddition(dkb.bucketIndex, success)
	}

	// If add failed due to full bucket, try resizing
	if !success && dkb.densityEstimator != nil {
		newSize := dkb.densityEstimator.SuggestBucketSize(dkb.bucketIndex)
		if newSize > dkb.maxSize {
			dkb.resize(newSize)
			// Retry the add after resize
			success = dkb.KBucket.AddNode(node)
		}
	}

	return success
}

// resize adjusts the bucket's maximum size.
func (dkb *DynamicKBucket) resize(newSize int) {
	dkb.mu.Lock()
	defer dkb.mu.Unlock()

	if newSize <= dkb.maxSize {
		return // Don't shrink
	}

	dkb.maxSize = newSize
	// Pre-allocate capacity for the new size
	if cap(dkb.nodes) < newSize {
		newNodes := make([]*Node, len(dkb.nodes), newSize)
		copy(newNodes, dkb.nodes)
		dkb.nodes = newNodes
	}
}

// GetMaxSize returns the current maximum size of the bucket.
func (dkb *DynamicKBucket) GetMaxSize() int {
	dkb.mu.RLock()
	defer dkb.mu.RUnlock()
	return dkb.maxSize
}

// GetCurrentSize returns the current number of nodes in the bucket.
func (dkb *DynamicKBucket) GetCurrentSize() int {
	dkb.mu.RLock()
	defer dkb.mu.RUnlock()
	return len(dkb.nodes)
}

// DynamicRoutingTable extends RoutingTable with dynamic bucket sizing.
type DynamicRoutingTable struct {
	*RoutingTable
	densityEstimator *DensityEstimator
	dynamicBuckets   [256]*DynamicKBucket
}

// NewDynamicRoutingTable creates a routing table with dynamic bucket sizing.
func NewDynamicRoutingTable(selfID crypto.ToxID, baseBucketSize int) *DynamicRoutingTable {
	estimator := NewDensityEstimator(baseBucketSize)

	drt := &DynamicRoutingTable{
		RoutingTable: &RoutingTable{
			selfID:       selfID,
			maxNodes:     baseBucketSize * 256, // Initial max, will grow dynamically
			groupStorage: NewGroupStorage(),
			relayStorage: NewRelayStorage(),
			lookupCache:  NewLookupCache(DefaultLookupCacheTTL, DefaultLookupCacheMaxSize),
		},
		densityEstimator: estimator,
	}

	// Initialize dynamic k-buckets
	for i := 0; i < 256; i++ {
		drt.dynamicBuckets[i] = NewDynamicKBucket(baseBucketSize, i, estimator)
		drt.RoutingTable.kBuckets[i] = &drt.dynamicBuckets[i].KBucket
	}

	return drt
}

// AddNode adds a node using dynamic bucket sizing.
func (drt *DynamicRoutingTable) AddNode(node *Node) bool {
	if node.ID.PublicKey == drt.selfID.PublicKey {
		return false // Don't add ourselves
	}

	// Calculate distance to determine bucket index
	selfNode := &Node{ID: drt.selfID}
	copy(selfNode.PublicKey[:], drt.selfID.PublicKey[:])

	dist := node.Distance(selfNode)
	bucketIndex := getBucketIndex(dist)

	drt.mu.Lock()
	added := drt.dynamicBuckets[bucketIndex].AddNodeDynamic(node)
	drt.mu.Unlock()

	// Invalidate cache if node was added
	if added && drt.lookupCache != nil {
		drt.lookupCache.Clear()
	}

	return added
}

// GetDensityStats returns the current density estimation statistics.
func (drt *DynamicRoutingTable) GetDensityStats() DensityStats {
	return drt.densityEstimator.Stats()
}

// GetBucketSizes returns the current size of each bucket.
func (drt *DynamicRoutingTable) GetBucketSizes() [256]int {
	var sizes [256]int
	drt.mu.RLock()
	defer drt.mu.RUnlock()

	for i := 0; i < 256; i++ {
		sizes[i] = drt.dynamicBuckets[i].GetMaxSize()
	}
	return sizes
}

// GetTotalCapacity returns the total capacity across all buckets.
func (drt *DynamicRoutingTable) GetTotalCapacity() int {
	drt.mu.RLock()
	defer drt.mu.RUnlock()

	total := 0
	for i := 0; i < 256; i++ {
		total += drt.dynamicBuckets[i].GetMaxSize()
	}
	return total
}

// GetTotalNodes returns the total number of nodes across all buckets.
func (drt *DynamicRoutingTable) GetTotalNodes() int {
	drt.mu.RLock()
	defer drt.mu.RUnlock()

	total := 0
	for i := 0; i < 256; i++ {
		total += drt.dynamicBuckets[i].GetCurrentSize()
	}
	return total
}
