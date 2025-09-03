package async

import "time"

// ConfigureMessageRetrieval sets the parameters for randomized message retrieval
// This allows users to customize the balance between privacy and efficiency
func (am *AsyncManager) ConfigureMessageRetrieval(
	baseInterval time.Duration,
	jitterPercent int,
	enableCoverTraffic bool,
	coverTrafficRatio float64,
) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Configure the client's retrieval scheduler
	am.client.ConfigureRetrieval(
		baseInterval,
		jitterPercent,
		enableCoverTraffic,
		coverTrafficRatio,
	)
}

// SetCoverTrafficEnabled enables or disables the use of cover traffic
// Cover traffic helps mask real usage patterns by sending dummy retrievals
func (am *AsyncManager) SetCoverTrafficEnabled(enabled bool) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.client.SetCoverTrafficEnabled(enabled)
}
