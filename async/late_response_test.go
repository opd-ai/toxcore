package async

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestSendResponseToChannel_AfterCleanup verifies that a late response
// delivered after cleanupResponseChannel has removed the channel entry is
// silently discarded without panicking (F-L1 regression guard).
func TestSendResponseToChannel_AfterCleanup(t *testing.T) {
	t.Parallel()

	ac := &AsyncClient{
		retrieveChannels: make(map[string]chan retrieveResponse),
	}

	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}

	responseChan := ac.setupResponseChannel(addr)

	// Simulate caller timing out and cleaning up before the response arrives.
	ac.cleanupResponseChannel(addr, responseChan)

	// A late sendResponseToChannel must not panic even though the channel is
	// no longer in the map (and was never closed).
	done := make(chan struct{})
	go func() {
		defer close(done)
		ac.sendResponseToChannel(responseChan, retrieveResponse{messages: nil})
	}()

	select {
	case <-done:
		// success — no panic, no block
	case <-time.After(time.Second):
		t.Fatal("sendResponseToChannel blocked unexpectedly after cleanup")
	}
}

// TestSendResponseToChannel_RacyTimeout stress-tests the timeout-then-late-
// delivery scenario under the race detector to confirm no data race exists.
func TestSendResponseToChannel_RacyTimeout(t *testing.T) {
	t.Parallel()

	ac := &AsyncClient{
		retrieveChannels: make(map[string]chan retrieveResponse),
	}

	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9998}

	const iterations = 200
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ch := ac.setupResponseChannel(addr)

			// Simulate concurrent timeout cleanup and late delivery.
			var inner sync.WaitGroup
			inner.Add(2)

			go func() {
				defer inner.Done()
				ac.cleanupResponseChannel(addr, ch)
			}()

			go func() {
				defer inner.Done()
				ac.sendResponseToChannel(ch, retrieveResponse{})
			}()

			inner.Wait()
		}()
	}

	wg.Wait()
}
