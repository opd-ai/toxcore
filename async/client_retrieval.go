package async

import (
	"time"
)

// StartScheduledRetrieval begins the randomized message retrieval schedule
// with configurable intervals and cover traffic. This helps prevent storage
// nodes from tracking user behavior through retrieval patterns.
func (ac *AsyncClient) StartScheduledRetrieval() {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	// Start the scheduler if it exists
	if ac.retrievalScheduler != nil {
		ac.retrievalScheduler.Start()
	}
}

// StopScheduledRetrieval stops the automated retrieval schedule
func (ac *AsyncClient) StopScheduledRetrieval() {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	// Stop the scheduler if it exists
	if ac.retrievalScheduler != nil {
		ac.retrievalScheduler.Stop()
	}
}

// ConfigureRetrieval updates the retrieval scheduler settings
func (ac *AsyncClient) ConfigureRetrieval(
	baseInterval time.Duration, 
	jitterPercent int,
	enableCoverTraffic bool,
	coverTrafficRatio float64,
) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	if ac.retrievalScheduler != nil {
		ac.retrievalScheduler.Configure(
			baseInterval,
			jitterPercent,
			enableCoverTraffic,
			coverTrafficRatio,
		)
	}
}

// SetCoverTrafficEnabled enables or disables cover traffic in the retrieval scheduler
func (ac *AsyncClient) SetCoverTrafficEnabled(enabled bool) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	if ac.retrievalScheduler != nil {
		ac.retrievalScheduler.SetCoverTrafficEnabled(enabled)
	}
}
