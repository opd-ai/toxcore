package messaging

import (
	"sync"
	"time"
)

// DisappearingMessageConfig holds the disappearing-message timer settings for
// a single conversation.  Both peers must share the same configuration for the
// feature to be meaningful; the peer that changes the timer sends a
// [MessageTypeDisappearingConfig] control message so the remote side can
// synchronise.
type DisappearingMessageConfig struct {
	// Enabled reports whether disappearing messages are active.
	Enabled bool `json:"enabled"`
	// Timer is the duration after which a received (or sent) message is deleted
	// from local storage.  Meaningful values range from 30 seconds to 4 weeks.
	// A zero value is treated as disabled.
	Timer time.Duration `json:"timer_ns"`
	// SetAt records when this configuration was last changed.  It is included in
	// the control message so both sides can resolve concurrent changes by taking
	// the most recent SetAt.
	SetAt time.Time `json:"set_at"`
}

// MessageTypeDisappearingConfig is a control message that carries a
// [DisappearingMessageConfig] to the remote peer so both sides share the same
// timer value.
const MessageTypeDisappearingConfig MessageType = 2

// disappearingEntry records a scheduled deletion.
type disappearingEntry struct {
	messageID uint32
	timer     *time.Timer
}

// DisappearingMessageManager schedules per-message deletion timers for a
// single conversation (identified by friendID).
//
// It is embedded inside [MessageManager] and serialised through the manager's
// own mutex, so callers must hold mm.mu before calling any method.
type DisappearingMessageManager struct {
	config  DisappearingMessageConfig
	entries map[uint32]*disappearingEntry // keyed by message ID
	mu      sync.Mutex
}

// newDisappearingMessageManager creates an empty manager with disappearing
// messages disabled.
func newDisappearingMessageManager() *DisappearingMessageManager {
	return &DisappearingMessageManager{
		entries: make(map[uint32]*disappearingEntry),
	}
}

// Config returns the current disappearing-message configuration.
func (d *DisappearingMessageManager) Config() DisappearingMessageConfig {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config
}

// SetConfig replaces the current configuration.  The caller is responsible for
// persisting the change and notifying the remote peer via a control message.
// If Enabled is false or Timer is zero, any pending timers are cancelled.
func (d *DisappearingMessageManager) SetConfig(cfg DisappearingMessageConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = cfg
	if !cfg.Enabled || cfg.Timer == 0 {
		d.cancelAllLocked()
	}
}

// ScheduleDeletion starts a timer that calls onDelete(messageID) after the
// configured timer duration.  If disappearing messages are disabled, the call
// is a no-op.  A second call for the same messageID replaces the previous timer.
func (d *DisappearingMessageManager) ScheduleDeletion(messageID uint32, onDelete func(uint32)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.config.Enabled || d.config.Timer == 0 {
		return
	}
	// Cancel any existing timer for this message.
	if e, ok := d.entries[messageID]; ok {
		e.timer.Stop()
	}
	t := time.AfterFunc(d.config.Timer, func() {
		d.mu.Lock()
		delete(d.entries, messageID)
		d.mu.Unlock()
		onDelete(messageID)
	})
	d.entries[messageID] = &disappearingEntry{messageID: messageID, timer: t}
}

// CancelDeletion stops the pending timer for a specific message.  It is a
// no-op if no timer is scheduled for messageID.
func (d *DisappearingMessageManager) CancelDeletion(messageID uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.entries[messageID]; ok {
		e.timer.Stop()
		delete(d.entries, messageID)
	}
}

// cancelAllLocked cancels all pending timers.  Must be called with d.mu held.
func (d *DisappearingMessageManager) cancelAllLocked() {
	for id, e := range d.entries {
		e.timer.Stop()
		delete(d.entries, id)
	}
}

// Stop cancels all pending timers and must be called when the conversation is
// closed or the application is shutting down to prevent goroutine leaks.
func (d *DisappearingMessageManager) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cancelAllLocked()
}
