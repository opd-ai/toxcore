package async

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

// RetrievalScheduler manages randomized retrieval schedules with cover traffic
// to prevent storage nodes from tracking user activity based on retrieval patterns
type RetrievalScheduler struct {
	mutex               sync.Mutex
	client              *AsyncClient
	running             bool
	stopChan            chan struct{}
	baseInterval        time.Duration // Base interval between retrievals
	jitterPercent       int           // Random jitter as percentage of base interval
	coverTrafficEnabled bool          // Whether to send cover traffic
	coverTrafficRatio   float64       // Ratio of cover traffic to real retrievals (0.0-1.0)
	
	lastRetrieval       time.Time     // When the last retrieval happened
	consecutiveEmpty    int           // Count of consecutive empty retrievals
}

// NewRetrievalScheduler creates a new scheduler with default settings
func NewRetrievalScheduler(client *AsyncClient) *RetrievalScheduler {
	return &RetrievalScheduler{
		client:              client,
		baseInterval:        5 * time.Minute,    // Default: check every 5 minutes
		jitterPercent:       50,                 // Add up to 50% random jitter to timing
		coverTrafficEnabled: true,               // Enable cover traffic by default
		coverTrafficRatio:   0.3,                // ~30% of retrievals will be cover traffic
		lastRetrieval:       time.Time{},        // Zero time
		consecutiveEmpty:    0,
		stopChan:            make(chan struct{}),
	}
}

// Start begins the randomized retrieval schedule
func (rs *RetrievalScheduler) Start() {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	if rs.running {
		return // Already running
	}
	
	rs.running = true
	rs.stopChan = make(chan struct{})
	
	go rs.retrievalLoop()
}

// Stop halts the retrieval schedule
func (rs *RetrievalScheduler) Stop() {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	if !rs.running {
		return
	}
	
	rs.running = false
	close(rs.stopChan)
}

// retrievalLoop runs the main retrieval scheduling loop
func (rs *RetrievalScheduler) retrievalLoop() {
	for {
		// Calculate next retrieval time with jitter
		nextInterval := rs.calculateNextInterval()
		
		// Wait until next retrieval time or stop signal
		select {
		case <-time.After(nextInterval):
			rs.performRetrieval()
		case <-rs.stopChan:
			return
		}
	}
}

// calculateNextInterval determines when the next retrieval should happen
// using base interval + randomized jitter to make timing unpredictable
func (rs *RetrievalScheduler) calculateNextInterval() time.Duration {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	// Base interval is adaptive based on activity
	interval := rs.baseInterval
	
	// If we've had multiple empty retrievals, gradually increase the interval
	if rs.consecutiveEmpty > 3 {
		// Exponential backoff, up to 4x the base interval
		multiplier := float64(rs.consecutiveEmpty-2)
		if multiplier > 4 {
			multiplier = 4
		}
		interval = time.Duration(float64(interval) * multiplier)
	}
	
	// Calculate jitter value (±jitterPercent% of interval)
	maxJitter := int64(float64(interval) * float64(rs.jitterPercent) / 100.0)
	
	// Generate random jitter between -maxJitter and +maxJitter
	jitterBig, _ := rand.Int(rand.Reader, big.NewInt(2*maxJitter))
	jitter := time.Duration(jitterBig.Int64() - maxJitter)
	
	// Apply jitter to base interval
	return interval + jitter
}

// performRetrieval executes a message retrieval operation
func (rs *RetrievalScheduler) performRetrieval() {
	rs.mutex.Lock()
	
	// Determine if this should be real or cover traffic
	isCoverTraffic := rs.shouldSendCoverTraffic()
	
	// Track retrieval time
	rs.lastRetrieval = time.Now()
	rs.mutex.Unlock()
	
	if isCoverTraffic {
		// For cover traffic, we make a retrieval but discard the results
		// This looks the same to the storage node as a real retrieval
		_, _ = rs.client.RetrieveObfuscatedMessages()
		return
	}
	
	// Real retrieval - process messages
	messages, err := rs.client.RetrieveObfuscatedMessages()
	
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	if err != nil || len(messages) == 0 {
		rs.consecutiveEmpty++
	} else {
		// Reset the counter when we get messages
		rs.consecutiveEmpty = 0
	}
}

// shouldSendCoverTraffic determines if the current retrieval should be cover traffic
func (rs *RetrievalScheduler) shouldSendCoverTraffic() bool {
	if !rs.coverTrafficEnabled {
		return false
	}
	
	// Generate random number between 0 and 1
	randomBig, _ := rand.Int(rand.Reader, big.NewInt(1000))
	random := float64(randomBig.Int64()) / 1000.0
	
	// Return true with probability equal to coverTrafficRatio
	return random < rs.coverTrafficRatio
}

// Configure updates the scheduler configuration
func (rs *RetrievalScheduler) Configure(
	baseInterval time.Duration,
	jitterPercent int,
	enableCoverTraffic bool,
	coverTrafficRatio float64,
) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	rs.baseInterval = baseInterval
	rs.jitterPercent = jitterPercent
	rs.coverTrafficEnabled = enableCoverTraffic
	rs.coverTrafficRatio = coverTrafficRatio
}

// SetCoverTrafficEnabled turns cover traffic on or off
func (rs *RetrievalScheduler) SetCoverTrafficEnabled(enabled bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	rs.coverTrafficEnabled = enabled
}
