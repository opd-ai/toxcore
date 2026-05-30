package messaging

import (
	"sync"
	"testing"
	"time"
)

// TestDisappearingMessageConfigDefaults verifies zero-value config is disabled.
func TestDisappearingMessageConfigDefaults(t *testing.T) {
	d := newDisappearingMessageManager()
	cfg := d.Config()
	if cfg.Enabled {
		t.Fatal("default config should have Enabled=false")
	}
	if cfg.Timer != 0 {
		t.Fatalf("default config should have Timer=0, got %v", cfg.Timer)
	}
}

// TestDisappearingMessageSetAndGetConfig round-trips a config through
// SetConfig / Config.
func TestDisappearingMessageSetAndGetConfig(t *testing.T) {
	d := newDisappearingMessageManager()
	want := DisappearingMessageConfig{
		Enabled: true,
		Timer:   30 * time.Second,
		SetAt:   time.Now().Truncate(time.Second),
	}
	d.SetConfig(want)
	got := d.Config()
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled: got %v, want %v", got.Enabled, want.Enabled)
	}
	if got.Timer != want.Timer {
		t.Errorf("Timer: got %v, want %v", got.Timer, want.Timer)
	}
}

// TestScheduleDeletionFires verifies the timer calls onDelete after the
// configured duration.
func TestScheduleDeletionFires(t *testing.T) {
	d := newDisappearingMessageManager()
	d.SetConfig(DisappearingMessageConfig{Enabled: true, Timer: 50 * time.Millisecond})

	var fired uint32
	var mu sync.Mutex
	d.ScheduleDeletion(42, func(id uint32) {
		mu.Lock()
		fired = id
		mu.Unlock()
	})

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	got := fired
	mu.Unlock()
	if got != 42 {
		t.Fatalf("onDelete not called with correct id: got %d, want 42", got)
	}
}

// TestScheduleDeletionDisabledIsNoop verifies that when the config is disabled
// the onDelete callback is never called.
func TestScheduleDeletionDisabledIsNoop(t *testing.T) {
	d := newDisappearingMessageManager()
	// Disabled by default — ScheduleDeletion should be a no-op.
	called := false
	d.ScheduleDeletion(99, func(_ uint32) { called = true })
	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("onDelete should not be called when disappearing messages are disabled")
	}
}

// TestCancelDeletionPreventsFiring verifies CancelDeletion stops the timer.
func TestCancelDeletionPreventsFiring(t *testing.T) {
	d := newDisappearingMessageManager()
	d.SetConfig(DisappearingMessageConfig{Enabled: true, Timer: 200 * time.Millisecond})

	called := false
	d.ScheduleDeletion(7, func(_ uint32) { called = true })
	d.CancelDeletion(7)

	time.Sleep(400 * time.Millisecond)
	if called {
		t.Fatal("onDelete should not fire after CancelDeletion")
	}
}

// TestStopCancelsAllTimers verifies Stop() cancels all pending timers.
func TestStopCancelsAllTimers(t *testing.T) {
	d := newDisappearingMessageManager()
	d.SetConfig(DisappearingMessageConfig{Enabled: true, Timer: 300 * time.Millisecond})

	var count int
	var mu sync.Mutex
	for i := uint32(1); i <= 5; i++ {
		d.ScheduleDeletion(i, func(_ uint32) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}
	d.Stop()

	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	got := count
	mu.Unlock()
	if got != 0 {
		t.Fatalf("after Stop(), %d timers still fired", got)
	}
}

// TestDisableConfigCancelsTimers ensures that calling SetConfig with
// Enabled=false cancels all in-flight timers.
func TestDisableConfigCancelsTimers(t *testing.T) {
	d := newDisappearingMessageManager()
	d.SetConfig(DisappearingMessageConfig{Enabled: true, Timer: 300 * time.Millisecond})

	called := false
	d.ScheduleDeletion(55, func(_ uint32) { called = true })

	// Disable before the timer fires.
	d.SetConfig(DisappearingMessageConfig{Enabled: false})
	time.Sleep(500 * time.Millisecond)
	if called {
		t.Fatal("timer should have been cancelled when disappearing was disabled")
	}
}

// TestMessageManagerSetAndGetDisappearingConfig tests the MessageManager API.
func TestMessageManagerSetAndGetDisappearingConfig(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	cfg := DisappearingMessageConfig{Enabled: true, Timer: time.Minute, SetAt: time.Now()}
	mm.SetDisappearingConfig(1, cfg)
	got := mm.GetDisappearingConfig(1)
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
	if got.Timer != time.Minute {
		t.Errorf("expected Timer=1m, got %v", got.Timer)
	}
}

// TestMessageManagerDeleteMessage verifies DeleteMessage removes the message
// from the in-memory store.
func TestMessageManagerDeleteMessage(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	msg := NewMessage(1, "hello", MessageTypeNormal)
	mm.mu.Lock()
	msg.ID = mm.nextID
	mm.nextID++
	mm.messages[msg.ID] = msg
	mm.mu.Unlock()

	id := msg.ID
	mm.DeleteMessage(id)

	if _, err := mm.GetMessage(id); err == nil {
		t.Fatal("message should be gone after DeleteMessage, but GetMessage returned nil error")
	}
}

// TestScheduleMessageDeletionIntegration wires ScheduleMessageDeletion to
// DeleteMessage and verifies the message disappears after the timer fires.
func TestScheduleMessageDeletionIntegration(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	mm.SetDisappearingConfig(1, DisappearingMessageConfig{Enabled: true, Timer: 50 * time.Millisecond})

	msg := NewMessage(1, "vanish", MessageTypeNormal)
	mm.mu.Lock()
	msg.ID = mm.nextID
	mm.nextID++
	mm.messages[msg.ID] = msg
	mm.mu.Unlock()

	id := msg.ID
	mm.ScheduleMessageDeletion(1, id, func(msgID uint32) {
		mm.DeleteMessage(msgID)
	})

	time.Sleep(200 * time.Millisecond)
	if _, err := mm.GetMessage(id); err == nil {
		t.Fatal("message should have been deleted by the disappearing timer")
	}
}

// TestTimerChangeMidConversation verifies that updating the timer mid-conversation
// affects newly scheduled timers but does not disturb already-fired ones.
func TestTimerChangeMidConversation(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	// Start with a long timer.
	mm.SetDisappearingConfig(1, DisappearingMessageConfig{Enabled: true, Timer: 10 * time.Second})

	msg1 := NewMessage(1, "msg1", MessageTypeNormal)
	mm.mu.Lock()
	msg1.ID = mm.nextID
	mm.nextID++
	mm.messages[msg1.ID] = msg1
	mm.mu.Unlock()

	// Schedule with the long timer — should not fire during the test.
	mm.ScheduleMessageDeletion(1, msg1.ID, func(id uint32) { mm.DeleteMessage(id) })

	// Change to a short timer.
	mm.SetDisappearingConfig(1, DisappearingMessageConfig{Enabled: true, Timer: 50 * time.Millisecond})

	msg2 := NewMessage(1, "msg2", MessageTypeNormal)
	mm.mu.Lock()
	msg2.ID = mm.nextID
	mm.nextID++
	mm.messages[msg2.ID] = msg2
	mm.mu.Unlock()

	mm.ScheduleMessageDeletion(1, msg2.ID, func(id uint32) { mm.DeleteMessage(id) })

	time.Sleep(200 * time.Millisecond)

	// msg1 should still be present (long timer).
	if _, err := mm.GetMessage(msg1.ID); err != nil {
		t.Error("msg1 should still be present (long timer was not changed retroactively)")
	}
	// msg2 should be gone (short timer).
	if _, err := mm.GetMessage(msg2.ID); err == nil {
		t.Error("msg2 should have been deleted by the short timer")
	}

	// Cleanup: cancel msg1's long timer.
	mm.mu.Lock()
	if d, ok := mm.disappearing[1]; ok {
		d.CancelDeletion(msg1.ID)
	}
	mm.mu.Unlock()
}
