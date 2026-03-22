// Package async provides asynchronous messaging with push-based notifications.
//
// This file implements a notification subscription system that enables storage
// nodes to push message arrival notifications to recipients, reducing latency
// from 30s polling to near-instant delivery when recipients come online.
package async

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// NotificationType identifies different notification events.
type NotificationType uint8

const (
	// NotifyMessageArrived indicates a new message is available.
	NotifyMessageArrived NotificationType = iota
	// NotifyMessageExpiring indicates a message will expire soon.
	NotifyMessageExpiring
	// NotifyStorageCapacity indicates storage capacity warning.
	NotifyStorageCapacity
	// NotifyPreKeyRequest indicates a pre-key exchange is requested.
	NotifyPreKeyRequest
)

// NotificationConfig holds configuration for the notification system.
type NotificationConfig struct {
	// MaxPendingNotifications limits queue size per subscriber.
	MaxPendingNotifications int
	// NotificationTimeout is the timeout for delivering notifications.
	NotificationTimeout time.Duration
	// EnableBatching groups notifications to reduce overhead.
	EnableBatching bool
	// BatchWindow is the time window for batching notifications.
	BatchWindow time.Duration
	// RetryAttempts is the number of delivery retry attempts.
	RetryAttempts int
	// RetryDelay is the delay between retry attempts.
	RetryDelay time.Duration
}

// DefaultNotificationConfig returns sensible defaults.
func DefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		MaxPendingNotifications: 1000,
		NotificationTimeout:     5 * time.Second,
		EnableBatching:          true,
		BatchWindow:             100 * time.Millisecond,
		RetryAttempts:           3,
		RetryDelay:              500 * time.Millisecond,
	}
}

// Notification represents a push notification from a storage node.
type Notification struct {
	Type        NotificationType
	SenderPK    [32]byte
	RecipientPK [32]byte
	MessageID   string
	Timestamp   time.Time
	Priority    uint8
	Metadata    map[string]string
}

// NotificationHandler processes incoming notifications.
type NotificationHandler func(*Notification) error

// Subscriber represents a notification subscriber.
type Subscriber struct {
	PublicKey  [32]byte
	Handler    NotificationHandler
	Queue      chan *Notification
	Active     atomic.Bool
	LastActive atomic.Int64
	Delivered  atomic.Uint64
	Dropped    atomic.Uint64
}

// NotificationHub manages notification subscriptions and delivery.
//
//export ToxNotificationHub
type NotificationHub struct {
	subscribers map[[32]byte]*Subscriber
	mu          sync.RWMutex
	config      NotificationConfig
	ctx         context.Context
	cancel      context.CancelFunc
	running     atomic.Bool
	wg          sync.WaitGroup
	stats       NotificationStats
}

// NotificationStats tracks hub statistics.
type NotificationStats struct {
	Subscriptions     atomic.Int64
	TotalDelivered    atomic.Uint64
	TotalDropped      atomic.Uint64
	TotalRetries      atomic.Uint64
	ActiveSubscribers atomic.Int64
}

// NewNotificationHub creates a new notification hub.
//
//export ToxNewNotificationHub
func NewNotificationHub(config *NotificationConfig) *NotificationHub {
	if config == nil {
		defaultConfig := DefaultNotificationConfig()
		config = &defaultConfig
	}

	ctx, cancel := context.WithCancel(context.Background())

	hub := &NotificationHub{
		subscribers: make(map[[32]byte]*Subscriber),
		config:      *config,
		ctx:         ctx,
		cancel:      cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":         "NewNotificationHub",
		"max_pending":      config.MaxPendingNotifications,
		"timeout":          config.NotificationTimeout,
		"batching_enabled": config.EnableBatching,
	}).Info("Created notification hub")

	return hub
}

// Subscribe registers a handler for notifications to the given public key.
//
//export ToxNotificationHubSubscribe
func (h *NotificationHub) Subscribe(publicKey [32]byte, handler NotificationHandler) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.subscribers[publicKey]; exists {
		// Update existing subscriber
		h.subscribers[publicKey].Handler = handler
		h.subscribers[publicKey].Active.Store(true)
		h.subscribers[publicKey].LastActive.Store(time.Now().UnixNano())
		return nil
	}

	subscriber := &Subscriber{
		PublicKey: publicKey,
		Handler:   handler,
		Queue:     make(chan *Notification, h.config.MaxPendingNotifications),
	}
	subscriber.Active.Store(true)
	subscriber.LastActive.Store(time.Now().UnixNano())

	h.subscribers[publicKey] = subscriber
	h.stats.Subscriptions.Add(1)
	h.stats.ActiveSubscribers.Add(1)

	// Start delivery goroutine for this subscriber
	h.wg.Add(1)
	go h.deliveryLoop(subscriber)

	logrus.WithFields(logrus.Fields{
		"function":   "Subscribe",
		"public_key": formatKey(publicKey),
	}).Debug("Subscriber registered")

	return nil
}

// Unsubscribe removes a notification subscription.
//
//export ToxNotificationHubUnsubscribe
func (h *NotificationHub) Unsubscribe(publicKey [32]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subscriber, exists := h.subscribers[publicKey]; exists {
		subscriber.Active.Store(false)
		close(subscriber.Queue)
		delete(h.subscribers, publicKey)
		h.stats.ActiveSubscribers.Add(-1)

		logrus.WithFields(logrus.Fields{
			"function":   "Unsubscribe",
			"public_key": formatKey(publicKey),
		}).Debug("Subscriber unregistered")
	}
}

// Notify sends a notification to a specific recipient.
//
//export ToxNotificationHubNotify
func (h *NotificationHub) Notify(notification *Notification) bool {
	h.mu.RLock()
	subscriber, exists := h.subscribers[notification.RecipientPK]
	h.mu.RUnlock()

	if !exists || !subscriber.Active.Load() {
		return false
	}

	select {
	case subscriber.Queue <- notification:
		return true
	default:
		// Queue full, drop notification
		subscriber.Dropped.Add(1)
		h.stats.TotalDropped.Add(1)
		logrus.WithFields(logrus.Fields{
			"function":   "Notify",
			"recipient":  formatKey(notification.RecipientPK),
			"queue_full": true,
		}).Debug("Notification dropped")
		return false
	}
}

// NotifyAll sends a notification to all subscribers matching a filter.
func (h *NotificationHub) NotifyAll(notification *Notification, filter func([32]byte) bool) int {
	h.mu.RLock()
	targets := make([]*Subscriber, 0)
	for pk, sub := range h.subscribers {
		if filter == nil || filter(pk) {
			targets = append(targets, sub)
		}
	}
	h.mu.RUnlock()

	delivered := 0
	for _, sub := range targets {
		notifCopy := *notification
		notifCopy.RecipientPK = sub.PublicKey
		if h.notifySubscriber(sub, &notifCopy) {
			delivered++
		}
	}

	return delivered
}

// notifySubscriber attempts to deliver a notification to a specific subscriber.
func (h *NotificationHub) notifySubscriber(sub *Subscriber, notification *Notification) bool {
	if !sub.Active.Load() {
		return false
	}

	select {
	case sub.Queue <- notification:
		return true
	default:
		sub.Dropped.Add(1)
		h.stats.TotalDropped.Add(1)
		return false
	}
}

// deliveryLoop processes notifications for a subscriber.
func (h *NotificationHub) deliveryLoop(sub *Subscriber) {
	defer h.wg.Done()

	var batch []*Notification
	var batchTimer <-chan time.Time

	for {
		select {
		case <-h.ctx.Done():
			h.flushBatch(sub, batch)
			return

		case notification, ok := <-sub.Queue:
			if !ok {
				h.flushBatch(sub, batch)
				return
			}

			if h.config.EnableBatching {
				batch = append(batch, notification)
				if batchTimer == nil {
					batchTimer = time.After(h.config.BatchWindow)
				}

				// Flush if batch is full
				if len(batch) >= 10 {
					h.flushBatch(sub, batch)
					batch = nil
					batchTimer = nil
				}
			} else {
				h.deliverNotification(sub, notification)
			}

		case <-batchTimer:
			h.flushBatch(sub, batch)
			batch = nil
			batchTimer = nil
		}
	}
}

// flushBatch delivers all notifications in the batch.
func (h *NotificationHub) flushBatch(sub *Subscriber, batch []*Notification) {
	for _, notification := range batch {
		h.deliverNotification(sub, notification)
	}
}

// deliverNotification delivers a single notification with retry.
func (h *NotificationHub) deliverNotification(sub *Subscriber, notification *Notification) {
	if sub.Handler == nil {
		return
	}

	var lastErr error
	for attempt := 0; attempt <= h.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			h.stats.TotalRetries.Add(1)
			time.Sleep(h.config.RetryDelay)
		}

		ctx, cancel := context.WithTimeout(h.ctx, h.config.NotificationTimeout)
		err := h.deliverWithTimeout(ctx, sub, notification)
		cancel()

		if err == nil {
			sub.Delivered.Add(1)
			h.stats.TotalDelivered.Add(1)
			sub.LastActive.Store(time.Now().UnixNano())
			return
		}
		lastErr = err
	}

	logrus.WithFields(logrus.Fields{
		"function":  "deliverNotification",
		"recipient": formatKey(sub.PublicKey),
		"error":     lastErr.Error(),
	}).Debug("Failed to deliver notification after retries")
}

// deliverWithTimeout delivers a notification with a timeout.
func (h *NotificationHub) deliverWithTimeout(ctx context.Context, sub *Subscriber, notification *Notification) error {
	done := make(chan error, 1)

	go func() {
		done <- sub.Handler(notification)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start starts the notification hub background processing.
//
//export ToxNotificationHubStart
func (h *NotificationHub) Start() {
	if h.running.Swap(true) {
		return
	}

	logrus.WithField("function", "Start").Info("Notification hub started")
}

// Stop stops the notification hub and waits for pending deliveries.
//
//export ToxNotificationHubStop
func (h *NotificationHub) Stop() {
	if !h.running.Swap(false) {
		return
	}

	h.cancel()
	h.wg.Wait()

	logrus.WithField("function", "Stop").Info("Notification hub stopped")
}

// SubscriberCount returns the number of active subscribers.
func (h *NotificationHub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// IsSubscribed checks if a public key is subscribed.
func (h *NotificationHub) IsSubscribed(publicKey [32]byte) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.subscribers[publicKey]
	return exists
}

// Stats returns hub statistics.
func (h *NotificationHub) Stats() (subscriptions, delivered, dropped, retries uint64, active int64) {
	return uint64(h.stats.Subscriptions.Load()),
		h.stats.TotalDelivered.Load(),
		h.stats.TotalDropped.Load(),
		h.stats.TotalRetries.Load(),
		h.stats.ActiveSubscribers.Load()
}

// formatKey formats a public key for logging.
func formatKey(key [32]byte) string {
	return string(key[:8])
}

// --- Integration with AsyncManager ---

// EnablePushNotifications enables push-based notifications on the AsyncManager.
func (am *AsyncManager) EnablePushNotifications(config *NotificationConfig) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.notificationHub == nil {
		am.notificationHub = NewNotificationHub(config)
		am.notificationHub.Start()
	}

	logrus.WithField("function", "EnablePushNotifications").Info("Push notifications enabled")
}

// DisablePushNotifications disables push-based notifications.
func (am *AsyncManager) DisablePushNotifications() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.notificationHub != nil {
		am.notificationHub.Stop()
		am.notificationHub = nil
	}
}

// SubscribeNotifications subscribes a friend to receive push notifications.
func (am *AsyncManager) SubscribeNotifications(friendPK [32]byte, handler NotificationHandler) error {
	am.mutex.RLock()
	hub := am.notificationHub
	am.mutex.RUnlock()

	if hub == nil {
		return nil // Silently ignore if push not enabled
	}

	return hub.Subscribe(friendPK, handler)
}

// UnsubscribeNotifications removes a friend's notification subscription.
func (am *AsyncManager) UnsubscribeNotifications(friendPK [32]byte) {
	am.mutex.RLock()
	hub := am.notificationHub
	am.mutex.RUnlock()

	if hub != nil {
		hub.Unsubscribe(friendPK)
	}
}

// NotifyMessageAvailable notifies a recipient that a message is available.
// This is called by storage nodes when they receive a message for storage.
func (am *AsyncManager) NotifyMessageAvailable(recipientPK, senderPK [32]byte, messageID string) {
	am.mutex.RLock()
	hub := am.notificationHub
	am.mutex.RUnlock()

	if hub == nil {
		return
	}

	notification := &Notification{
		Type:        NotifyMessageArrived,
		SenderPK:    senderPK,
		RecipientPK: recipientPK,
		MessageID:   messageID,
		Timestamp:   time.Now(),
		Priority:    0, // High priority
	}

	hub.Notify(notification)
}

// TriggerImmediateRetrieval triggers immediate message retrieval for a recipient.
// This is called when a push notification arrives.
func (am *AsyncManager) TriggerImmediateRetrieval(recipientPK [32]byte) {
	// Use a goroutine to avoid blocking the notification handler
	go func() {
		am.retrievePendingMessages()
	}()
}
