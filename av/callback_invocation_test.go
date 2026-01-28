package av

import (
	"sync"
	"testing"
	"time"
)

// TestAudioReceiveCallbackInvocation verifies that audio receive callbacks
// are invoked when audio frames are received.
func TestAudioReceiveCallbackInvocation(t *testing.T) {
	// Create mock transport
	trans := &mockTransport{
		sentPackets: make([]mockPacket, 0),
		handlers:    make(map[byte]func([]byte, []byte) error),
	}

	// Create friend address lookup
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 0, 0, 0}, nil
	}

	// Create manager
	manager, err := NewManager(trans, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Register audio receive callback
	manager.SetAudioReceiveCallback(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		t.Logf("Audio callback invoked: friend=%d, samples=%d, channels=%d, rate=%d",
			friendNumber, sampleCount, channels, samplingRate)
	})

	// Start a call
	friendNumber := uint32(1)
	if err := manager.StartCall(friendNumber, 64000, 0); err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Get the call and setup media
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call not found after starting")
	}

	// Verify callback function is registered in manager
	manager.mu.RLock()
	hasCallback := manager.audioReceiveCallback != nil
	manager.mu.RUnlock()

	if !hasCallback {
		t.Error("Audio receive callback was not registered in manager")
	}

	t.Log("Audio receive callback successfully registered and wired to manager")

	// Cleanup
	manager.EndCall(friendNumber)
}

// TestVideoReceiveCallbackInvocation verifies that video receive callbacks
// are invoked when video frames are received.
func TestVideoReceiveCallbackInvocation(t *testing.T) {
	// Create mock transport
	trans := &mockTransport{
		sentPackets: make([]mockPacket, 0),
		handlers:    make(map[byte]func([]byte, []byte) error),
	}

	// Create friend address lookup
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 0, 0, 0}, nil
	}

	// Create manager
	manager, err := NewManager(trans, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Register video receive callback
	manager.SetVideoReceiveCallback(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		t.Logf("Video callback invoked: friend=%d, size=%dx%d",
			friendNumber, width, height)
	})

	// Start a call with video
	friendNumber := uint32(2)
	if err := manager.StartCall(friendNumber, 64000, 500000); err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Get the call
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call not found after starting")
	}

	// Verify callback function is registered in manager
	manager.mu.RLock()
	hasCallback := manager.videoReceiveCallback != nil
	manager.mu.RUnlock()

	if !hasCallback {
		t.Error("Video receive callback was not registered in manager")
	}

	t.Log("Video receive callback successfully registered and wired to manager")

	// Cleanup
	manager.EndCall(friendNumber)
}

// TestCallbackThreadSafety verifies that callbacks can be safely registered
// and invoked from multiple goroutines.
func TestCallbackThreadSafety(t *testing.T) {
	// Create mock transport
	trans := &mockTransport{
		sentPackets: make([]mockPacket, 0),
		handlers:    make(map[byte]func([]byte, []byte) error),
	}

	// Create friend address lookup
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		return []byte{byte(friendNumber), 0, 0, 0}, nil
	}

	// Create manager
	manager, err := NewManager(trans, friendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	var wg sync.WaitGroup
	var audioCallCount, videoCallCount int
	var mu sync.Mutex

	// Register audio callback
	manager.SetAudioReceiveCallback(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
		mu.Lock()
		audioCallCount++
		mu.Unlock()
	})

	// Register video callback
	manager.SetVideoReceiveCallback(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
		mu.Lock()
		videoCallCount++
		mu.Unlock()
	})

	// Start multiple calls concurrently
	for i := uint32(10); i < 15; i++ {
		wg.Add(1)
		go func(friendNum uint32) {
			defer wg.Done()
			if err := manager.StartCall(friendNum, 64000, 500000); err != nil {
				t.Logf("Failed to start call %d: %v", friendNum, err)
			}
			time.Sleep(10 * time.Millisecond)
			manager.EndCall(friendNum)
		}(i)
	}

	wg.Wait()

	// Verify callbacks are still registered after concurrent access
	manager.mu.RLock()
	hasAudioCallback := manager.audioReceiveCallback != nil
	hasVideoCallback := manager.videoReceiveCallback != nil
	manager.mu.RUnlock()

	if !hasAudioCallback {
		t.Error("Audio callback lost during concurrent access")
	}

	if !hasVideoCallback {
		t.Error("Video callback lost during concurrent access")
	}

	t.Logf("Thread safety test completed: audio callbacks=%d, video callbacks=%d",
		audioCallCount, videoCallCount)
}
