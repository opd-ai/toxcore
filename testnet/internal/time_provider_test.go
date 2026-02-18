package internal

import (
	"testing"
	"time"
)

func TestTimeProvider(t *testing.T) {
	t.Run("DefaultTimeProvider returns system time", func(t *testing.T) {
		tp := NewDefaultTimeProvider()
		before := time.Now()
		now := tp.Now()
		after := time.Now()

		if now.Before(before) || now.After(after) {
			t.Errorf("DefaultTimeProvider.Now() returned %v, expected between %v and %v", now, before, after)
		}
	})

	t.Run("DefaultTimeProvider Since returns duration", func(t *testing.T) {
		tp := NewDefaultTimeProvider()
		past := time.Now().Add(-100 * time.Millisecond)
		since := tp.Since(past)

		if since < 100*time.Millisecond {
			t.Errorf("DefaultTimeProvider.Since() returned %v, expected >= 100ms", since)
		}
	})

	t.Run("MockTimeProvider returns controlled time", func(t *testing.T) {
		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)

		if !mock.Now().Equal(startTime) {
			t.Errorf("MockTimeProvider.Now() = %v, want %v", mock.Now(), startTime)
		}
	})

	t.Run("MockTimeProvider Advance works", func(t *testing.T) {
		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)

		mock.Advance(5 * time.Minute)
		expected := startTime.Add(5 * time.Minute)

		if !mock.Now().Equal(expected) {
			t.Errorf("MockTimeProvider.Now() after Advance = %v, want %v", mock.Now(), expected)
		}
	})

	t.Run("MockTimeProvider Set works", func(t *testing.T) {
		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		newTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)

		mock.Set(newTime)

		if !mock.Now().Equal(newTime) {
			t.Errorf("MockTimeProvider.Now() after Set = %v, want %v", mock.Now(), newTime)
		}
	})

	t.Run("MockTimeProvider Since calculates correctly", func(t *testing.T) {
		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)

		pastTime := startTime.Add(-10 * time.Second)
		since := mock.Since(pastTime)

		if since != 10*time.Second {
			t.Errorf("MockTimeProvider.Since() = %v, want %v", since, 10*time.Second)
		}
	})

	t.Run("getTimeProvider returns default when nil", func(t *testing.T) {
		result := getTimeProvider(nil)
		if result == nil {
			t.Error("getTimeProvider(nil) should not return nil")
		}

		// Verify it acts like the default provider
		before := time.Now()
		now := result.Now()
		after := time.Now()

		if now.Before(before) || now.After(after) {
			t.Error("getTimeProvider(nil) should return system time")
		}
	})

	t.Run("getTimeProvider returns provided when not nil", func(t *testing.T) {
		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)

		result := getTimeProvider(mock)

		if result != mock {
			t.Error("getTimeProvider should return the provided TimeProvider")
		}

		if !result.Now().Equal(startTime) {
			t.Errorf("result.Now() = %v, want %v", result.Now(), startTime)
		}
	})
}

func TestOrchestratorTimeProvider(t *testing.T) {
	t.Run("SetTimeProvider changes time source", func(t *testing.T) {
		config := DefaultTestConfig()
		orchestrator, err := NewTestOrchestrator(config)
		if err != nil {
			t.Fatalf("Failed to create orchestrator: %v", err)
		}
		defer orchestrator.Cleanup()

		startTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		mock := NewMockTimeProvider(startTime)
		orchestrator.SetTimeProvider(mock)

		// Verify the time provider is used
		now := orchestrator.getTimeProvider().Now()
		if !now.Equal(startTime) {
			t.Errorf("getTimeProvider().Now() = %v, want %v", now, startTime)
		}
	})
}

func TestBootstrapServerTimeProvider(t *testing.T) {
	t.Run("BootstrapServer has default TimeProvider", func(t *testing.T) {
		// Since we can't actually create a BootstrapServer without Tox,
		// we test that the struct has the timeProvider field and getTimeProvider works
		bs := &BootstrapServer{}
		tp := bs.getTimeProvider()
		if tp == nil {
			t.Error("BootstrapServer.getTimeProvider() should not return nil")
		}
	})
}
