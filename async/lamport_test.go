package async

import (
	"sync"
	"testing"

	toxcrypto "github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLamportClockBasic(t *testing.T) {
	clock := NewLamportClock()
	assert.Equal(t, uint64(0), clock.Current())

	// Tick increments
	ts1 := clock.Tick()
	assert.Equal(t, uint64(1), ts1)
	assert.Equal(t, uint64(1), clock.Current())

	ts2 := clock.Tick()
	assert.Equal(t, uint64(2), ts2)
}

func TestLamportClockUpdate(t *testing.T) {
	clock := NewLamportClock()
	clock.Tick() // 1
	clock.Tick() // 2

	// Update with lower value - should increment to 3
	newTs := clock.Update(1)
	assert.Equal(t, uint64(3), newTs)

	// Update with higher value - should jump to 11
	newTs = clock.Update(10)
	assert.Equal(t, uint64(11), newTs)

	// Verify current
	assert.Equal(t, uint64(11), clock.Current())
}

func TestLamportClockConcurrent(t *testing.T) {
	clock := NewLamportClock()
	var wg sync.WaitGroup
	iterations := 1000

	// Spawn multiple goroutines doing ticks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				clock.Tick()
			}
		}()
	}

	wg.Wait()

	// Should have exactly 10*1000 ticks
	assert.Equal(t, uint64(10*iterations), clock.Current())
}

func TestLamportClockSet(t *testing.T) {
	clock := NewLamportClock()
	clock.Set(100)
	assert.Equal(t, uint64(100), clock.Current())

	// Tick continues from set value
	ts := clock.Tick()
	assert.Equal(t, uint64(101), ts)
}

func TestLamportClockFrom(t *testing.T) {
	clock := NewLamportClockFrom(42)
	assert.Equal(t, uint64(42), clock.Current())
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b     uint64
		expected int
	}{
		{1, 2, -1},
		{2, 1, 1},
		{1, 1, 0},
		{0, 0, 0},
		{0, 1, -1},
	}

	for _, tt := range tests {
		result := Compare(tt.a, tt.b)
		assert.Equal(t, tt.expected, result, "Compare(%d, %d)", tt.a, tt.b)
	}
}

func TestMessageOrdering(t *testing.T) {
	mo := NewMessageOrdering()

	// Get timestamps for outgoing messages
	ts1 := mo.GetTimestamp()
	ts2 := mo.GetTimestamp()
	require.True(t, ts2 > ts1, "timestamps should be monotonically increasing")

	// Process incoming message with higher timestamp
	mo.ProcessIncoming(100)
	ts3 := mo.GetTimestamp()
	require.True(t, ts3 > 100, "timestamp after update should be > received")
}

func TestMessageOrderingFrom(t *testing.T) {
	mo := NewMessageOrderingFrom(50)
	assert.Equal(t, uint64(50), mo.CurrentClock())
}

func TestSortByLamport(t *testing.T) {
	type msg struct {
		text      string
		timestamp uint64
	}

	messages := []msg{
		{"third", 3},
		{"first", 1},
		{"second", 2},
		{"fourth", 4},
	}

	SortByLamport(messages, func(m msg) uint64 { return m.timestamp })

	assert.Equal(t, "first", messages[0].text)
	assert.Equal(t, "second", messages[1].text)
	assert.Equal(t, "third", messages[2].text)
	assert.Equal(t, "fourth", messages[3].text)
}

func TestSortByLamportStable(t *testing.T) {
	// Test stability for equal timestamps
	type msg struct {
		text      string
		timestamp uint64
		order     int // original order
	}

	messages := []msg{
		{"a", 1, 0},
		{"b", 1, 1},
		{"c", 1, 2},
	}

	SortByLamport(messages, func(m msg) uint64 { return m.timestamp })

	// Stable sort should preserve original order
	assert.Equal(t, "a", messages[0].text)
	assert.Equal(t, "b", messages[1].text)
	assert.Equal(t, "c", messages[2].text)
}

func TestFilterCausallyOrdered(t *testing.T) {
	type msg struct {
		text      string
		timestamp uint64
	}

	messages := []msg{
		{"old1", 1},
		{"old2", 5},
		{"new1", 10},
		{"new2", 15},
	}

	filtered := FilterCausallyOrdered(messages, 5, func(m msg) uint64 { return m.timestamp })

	require.Len(t, filtered, 2)
	assert.Equal(t, "new1", filtered[0].text)
	assert.Equal(t, "new2", filtered[1].text)
}

func TestLamportOrderingHappensBeforeRelationship(t *testing.T) {
	// Simulate two processes exchanging messages
	processA := NewMessageOrdering()
	processB := NewMessageOrdering()

	// Process A sends a message
	msgFromA := processA.GetTimestamp()

	// Process B receives and responds
	processB.ProcessIncoming(msgFromA)
	msgFromB := processB.GetTimestamp()

	// Verify happens-before: A's message < B's response
	require.True(t, msgFromA < msgFromB, "A's message should happen before B's response")

	// Process A receives B's response
	processA.ProcessIncoming(msgFromB)
	msgFromA2 := processA.GetTimestamp()

	// Verify: B's message < A's new message
	require.True(t, msgFromB < msgFromA2, "B's message should happen before A's new message")
}

func TestAsyncMessageLamportFields(t *testing.T) {
	// Verify the LamportClock and SenderClockHint fields work correctly
	msg := AsyncMessage{
		LamportClock:    42,
		SenderClockHint: 41,
	}

	assert.Equal(t, uint64(42), msg.LamportClock)
	assert.Equal(t, uint64(41), msg.SenderClockHint)
}

func TestRetrieveMessagesSortedByLamport(t *testing.T) {
	// Create a MessageStorage
	keyPair, err := toxcrypto.GenerateKeyPair()
	require.NoError(t, err)

	// Create temp directory
	tmpDir := t.TempDir()
	storage := NewMessageStorage(keyPair, tmpDir)

	// Create messages with different Lamport timestamps
	recipientPK := [32]byte{1, 2, 3}
	senderPK := [32]byte{4, 5, 6}
	nonce := [24]byte{7, 8, 9}
	encryptedData := []byte("test message data")

	// Store 4 messages and manually set their Lamport clocks
	timestamps := []uint64{5, 1, 10, 3}
	for i, ts := range timestamps {
		msgID, err := storage.StoreMessage(recipientPK, senderPK, encryptedData, nonce, MessageTypeNormal)
		require.NoError(t, err)

		// Manually update the message's LamportClock for testing
		storage.mutex.Lock()
		if msg, exists := storage.messages[msgID]; exists {
			msg.LamportClock = ts
		}
		storage.mutex.Unlock()
		_ = i // suppress unused variable
	}

	// Retrieve messages - should be sorted by Lamport clock
	retrieved, err := storage.RetrieveMessages(recipientPK)
	require.NoError(t, err)
	require.Len(t, retrieved, 4)

	// Verify order
	assert.Equal(t, uint64(1), retrieved[0].LamportClock)
	assert.Equal(t, uint64(3), retrieved[1].LamportClock)
	assert.Equal(t, uint64(5), retrieved[2].LamportClock)
	assert.Equal(t, uint64(10), retrieved[3].LamportClock)
}
