package async

import "sync"

// stopLoop marks a background loop as stopped and closes its stop channel once.
func stopLoop(mu sync.Locker, running *bool, stopChan *chan struct{}) bool {
	mu.Lock()
	defer mu.Unlock()
	if !*running {
		return false
	}
	*running = false
	close(*stopChan)
	return true
}
