package async

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultNotificationConfig(t *testing.T) {
	config := DefaultNotificationConfig()

	assert.Equal(t, 1000, config.MaxPendingNotifications)
	assert.Equal(t, 5*time.Second, config.NotificationTimeout)
	assert.True(t, config.EnableBatching)
	assert.Equal(t, 100*time.Millisecond, config.BatchWindow)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 500*time.Millisecond, config.RetryDelay)
}

func TestNewNotificationHub(t *testing.T) {
	hub := NewNotificationHub(nil)
	require.NotNil(t, hub)

	assert.Equal(t, 0, hub.SubscriberCount())
	assert.Equal(t, 1000, hub.config.MaxPendingNotifications)
}

func TestNotificationHubCustomConfig(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 500,
		NotificationTimeout:     2 * time.Second,
		EnableBatching:          false,
		BatchWindow:             50 * time.Millisecond,
		RetryAttempts:           1,
		RetryDelay:              100 * time.Millisecond,
	}

	hub := NewNotificationHub(config)
	require.NotNil(t, hub)

	assert.Equal(t, 500, hub.config.MaxPendingNotifications)
	assert.False(t, hub.config.EnableBatching)
}

func TestNotificationHubSubscribe(t *testing.T) {
	hub := NewNotificationHub(nil)
	hub.Start()
	defer hub.Stop()

	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_12345678901234"))

	var received atomic.Int32
	handler := func(n *Notification) error {
		received.Add(1)
		return nil
	}

	err := hub.Subscribe(publicKey, handler)
	assert.NoError(t, err)
	assert.Equal(t, 1, hub.SubscriberCount())
	assert.True(t, hub.IsSubscribed(publicKey))
}

func TestNotificationHubUnsubscribe(t *testing.T) {
	hub := NewNotificationHub(nil)
	hub.Start()
	defer hub.Stop()

	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_12345678901234"))

	handler := func(n *Notification) error { return nil }

	err := hub.Subscribe(publicKey, handler)
	assert.NoError(t, err)
	assert.Equal(t, 1, hub.SubscriberCount())

	hub.Unsubscribe(publicKey)
	assert.Equal(t, 0, hub.SubscriberCount())
	assert.False(t, hub.IsSubscribed(publicKey))
}

func TestNotificationHubNotify(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 100,
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
		RetryDelay:              10 * time.Millisecond,
	}

	hub := NewNotificationHub(config)
	hub.Start()
	defer hub.Stop()

	var recipientKey [32]byte
	copy(recipientKey[:], []byte("recipient_key_12345678901234567"))

	received := make(chan *Notification, 1)
	handler := func(n *Notification) error {
		received <- n
		return nil
	}

	err := hub.Subscribe(recipientKey, handler)
	require.NoError(t, err)

	notification := &Notification{
		Type:        NotifyMessageArrived,
		RecipientPK: recipientKey,
		MessageID:   "msg123",
		Timestamp:   time.Now(),
	}

	success := hub.Notify(notification)
	assert.True(t, success)

	select {
	case n := <-received:
		assert.Equal(t, NotifyMessageArrived, n.Type)
		assert.Equal(t, "msg123", n.MessageID)
	case <-time.After(2 * time.Second):
		t.Fatal("Notification not received")
	}
}

func TestNotificationHubNotifyNonexistent(t *testing.T) {
	hub := NewNotificationHub(nil)
	hub.Start()
	defer hub.Stop()

	var recipientKey [32]byte
	copy(recipientKey[:], []byte("nonexistent_key_12345678901234"))

	notification := &Notification{
		Type:        NotifyMessageArrived,
		RecipientPK: recipientKey,
		MessageID:   "msg123",
	}

	success := hub.Notify(notification)
	assert.False(t, success) // No subscriber
}

func TestNotificationHubNotifyAll(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 100,
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
	}

	hub := NewNotificationHub(config)
	hub.Start()
	defer hub.Stop()

	var received atomic.Int32

	// Subscribe 3 recipients
	for i := 0; i < 3; i++ {
		var key [32]byte
		key[0] = byte(i)
		handler := func(n *Notification) error {
			received.Add(1)
			return nil
		}
		err := hub.Subscribe(key, handler)
		require.NoError(t, err)
	}

	notification := &Notification{
		Type:      NotifyStorageCapacity,
		MessageID: "capacity_warning",
		Timestamp: time.Now(),
	}

	delivered := hub.NotifyAll(notification, nil) // No filter, notify all
	assert.Equal(t, 3, delivered)

	// Wait for deliveries
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(3), received.Load())
}

func TestNotificationHubNotifyAllWithFilter(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 100,
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
	}

	hub := NewNotificationHub(config)
	hub.Start()
	defer hub.Stop()

	var received atomic.Int32

	// Subscribe 5 recipients
	for i := 0; i < 5; i++ {
		var key [32]byte
		key[0] = byte(i)
		handler := func(n *Notification) error {
			received.Add(1)
			return nil
		}
		err := hub.Subscribe(key, handler)
		require.NoError(t, err)
	}

	notification := &Notification{
		Type:      NotifyMessageExpiring,
		MessageID: "expiring",
	}

	// Filter: only keys where first byte is even
	filter := func(pk [32]byte) bool {
		return pk[0]%2 == 0
	}

	delivered := hub.NotifyAll(notification, filter)
	assert.Equal(t, 3, delivered) // 0, 2, 4

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(3), received.Load())
}

func TestNotificationHubStats(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 100,
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
	}

	hub := NewNotificationHub(config)
	hub.Start()
	defer hub.Stop()

	var key [32]byte
	key[0] = 1

	handler := func(n *Notification) error { return nil }
	hub.Subscribe(key, handler)

	notification := &Notification{
		Type:        NotifyMessageArrived,
		RecipientPK: key,
	}

	hub.Notify(notification)
	hub.Notify(notification)

	time.Sleep(200 * time.Millisecond)

	subs, delivered, dropped, retries, active := hub.Stats()
	assert.Equal(t, uint64(1), subs)
	assert.True(t, delivered >= 2)
	assert.Equal(t, uint64(0), dropped)
	assert.Equal(t, uint64(0), retries)
	assert.Equal(t, int64(1), active)
}

func TestNotificationHubQueueFull(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 2, // Very small queue
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
	}

	hub := NewNotificationHub(config)
	// Don't start - handler won't consume

	var key [32]byte
	key[0] = 1

	blockingHandler := func(n *Notification) error {
		time.Sleep(10 * time.Second) // Block forever
		return nil
	}

	hub.Subscribe(key, blockingHandler)

	notification := &Notification{
		Type:        NotifyMessageArrived,
		RecipientPK: key,
	}

	// First 2 should succeed
	assert.True(t, hub.Notify(notification))
	assert.True(t, hub.Notify(notification))

	// Third should be dropped (queue full)
	assert.False(t, hub.Notify(notification))

	_, _, dropped, _, _ := hub.Stats()
	assert.Equal(t, uint64(1), dropped)
}

func TestNotificationHubConcurrency(t *testing.T) {
	config := &NotificationConfig{
		MaxPendingNotifications: 1000,
		NotificationTimeout:     1 * time.Second,
		EnableBatching:          false,
		RetryAttempts:           0,
	}

	hub := NewNotificationHub(config)
	hub.Start()
	defer hub.Stop()

	var received atomic.Int64
	numSubscribers := 10
	notificationsPerSubscriber := 100

	// Subscribe multiple recipients
	for i := 0; i < numSubscribers; i++ {
		var key [32]byte
		key[0] = byte(i)
		handler := func(n *Notification) error {
			received.Add(1)
			return nil
		}
		hub.Subscribe(key, handler)
	}

	// Send notifications concurrently
	var wg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(subscriberID int) {
			defer wg.Done()
			var key [32]byte
			key[0] = byte(subscriberID)

			for j := 0; j < notificationsPerSubscriber; j++ {
				notification := &Notification{
					Type:        NotifyMessageArrived,
					RecipientPK: key,
					MessageID:   "msg",
				}
				hub.Notify(notification)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// All notifications should be delivered
	assert.Equal(t, int64(numSubscribers*notificationsPerSubscriber), received.Load())
}

func TestNotificationHubStartStop(t *testing.T) {
	hub := NewNotificationHub(nil)

	// Start multiple times should be safe
	hub.Start()
	hub.Start()

	// Stop multiple times should be safe
	hub.Stop()
	hub.Stop()
}

func TestNotificationTypes(t *testing.T) {
	// Verify notification type values
	assert.Equal(t, NotificationType(0), NotifyMessageArrived)
	assert.Equal(t, NotificationType(1), NotifyMessageExpiring)
	assert.Equal(t, NotificationType(2), NotifyStorageCapacity)
	assert.Equal(t, NotificationType(3), NotifyPreKeyRequest)
}

func TestNotificationWithMetadata(t *testing.T) {
	notification := &Notification{
		Type:      NotifyMessageArrived,
		MessageID: "test123",
		Timestamp: time.Now(),
		Priority:  1,
		Metadata: map[string]string{
			"size":   "1024",
			"sender": "alice",
		},
	}

	assert.Equal(t, "1024", notification.Metadata["size"])
	assert.Equal(t, "alice", notification.Metadata["sender"])
}

func TestSubscriberResubscribe(t *testing.T) {
	hub := NewNotificationHub(nil)
	hub.Start()
	defer hub.Stop()

	var key [32]byte
	key[0] = 1

	var count1, count2 atomic.Int32

	handler1 := func(n *Notification) error {
		count1.Add(1)
		return nil
	}
	handler2 := func(n *Notification) error {
		count2.Add(1)
		return nil
	}

	// Subscribe with handler1
	hub.Subscribe(key, handler1)
	assert.Equal(t, 1, hub.SubscriberCount())

	// Re-subscribe with handler2 (should update handler)
	hub.Subscribe(key, handler2)
	assert.Equal(t, 1, hub.SubscriberCount()) // Still 1 subscriber

	notification := &Notification{
		Type:        NotifyMessageArrived,
		RecipientPK: key,
	}

	hub.Notify(notification)
	time.Sleep(200 * time.Millisecond)

	// Handler2 should receive, not handler1
	assert.Equal(t, int32(0), count1.Load())
	assert.Equal(t, int32(1), count2.Load())
}
