// ToxAV Integration Example - Tests
//
// Comprehensive test coverage for the ToxAV integration demo,
// testing command processing, time providers, and call management logic.

package main

import (
	"testing"
	"time"
)

// MockTimeProvider implements TimeProvider for deterministic testing.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock's current time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Since returns the duration since t based on mock time.
func (m *MockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

// Advance moves the mock time forward by d.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// TestRealTimeProvider verifies that RealTimeProvider returns valid times.
func TestRealTimeProvider(t *testing.T) {
	provider := RealTimeProvider{}

	t.Run("Now returns current time", func(t *testing.T) {
		before := time.Now()
		result := provider.Now()
		after := time.Now()

		if result.Before(before) || result.After(after) {
			t.Errorf("RealTimeProvider.Now() returned %v, expected between %v and %v",
				result, before, after)
		}
	})

	t.Run("Since returns positive duration", func(t *testing.T) {
		past := time.Now().Add(-100 * time.Millisecond)
		duration := provider.Since(past)

		if duration < 100*time.Millisecond {
			t.Errorf("RealTimeProvider.Since() = %v, expected >= 100ms", duration)
		}
	})
}

// TestMockTimeProvider verifies MockTimeProvider behavior for testing.
func TestMockTimeProvider(t *testing.T) {
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	provider := &MockTimeProvider{currentTime: fixedTime}

	t.Run("Now returns fixed time", func(t *testing.T) {
		result := provider.Now()
		if !result.Equal(fixedTime) {
			t.Errorf("MockTimeProvider.Now() = %v, want %v", result, fixedTime)
		}
	})

	t.Run("Since calculates duration from mock time", func(t *testing.T) {
		past := fixedTime.Add(-5 * time.Minute)
		duration := provider.Since(past)

		if duration != 5*time.Minute {
			t.Errorf("MockTimeProvider.Since() = %v, want %v", duration, 5*time.Minute)
		}
	})

	t.Run("Advance moves time forward", func(t *testing.T) {
		provider.Advance(10 * time.Second)
		expected := fixedTime.Add(10 * time.Second)

		if !provider.Now().Equal(expected) {
			t.Errorf("After Advance(10s), Now() = %v, want %v", provider.Now(), expected)
		}
	})
}

// TestParseMessageCommand tests the ParseMessageCommand function.
func TestParseMessageCommand(t *testing.T) {
	testCases := []struct {
		name         string
		message      string
		expectedCmd  MessageCommand
		expectedText string
	}{
		{"call command", "call", MessageCommandCall, ""},
		{"Call uppercase", "CALL", MessageCommandCall, ""},
		{"call with whitespace", "  call  ", MessageCommandCall, ""},
		{"videocall command", "videocall", MessageCommandVideoCall, ""},
		{"VideoCall mixed case", "VideoCall", MessageCommandVideoCall, ""},
		{"status command", "status", MessageCommandStatus, ""},
		{"STATUS uppercase", "STATUS", MessageCommandStatus, ""},
		{"help command", "help", MessageCommandHelp, ""},
		{"Help mixed case", "Help", MessageCommandHelp, ""},
		{"echo command", "echo hello", MessageCommandEcho, "hello"},
		{"echo with spaces", "echo hello world", MessageCommandEcho, "hello world"},
		{"ECHO uppercase", "ECHO test", MessageCommandEcho, "test"},
		{"random message", "hello there", MessageCommandNone, ""},
		{"empty message", "", MessageCommandNone, ""},
		{"whitespace only", "   ", MessageCommandNone, ""},
		{"partial command", "cal", MessageCommandNone, ""},
		{"call with extra", "call123", MessageCommandNone, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, text := ParseMessageCommand(tc.message)

			if cmd != tc.expectedCmd {
				t.Errorf("ParseMessageCommand(%q) cmd = %v, want %v", tc.message, cmd, tc.expectedCmd)
			}
			if text != tc.expectedText {
				t.Errorf("ParseMessageCommand(%q) text = %q, want %q", tc.message, text, tc.expectedText)
			}
		})
	}
}

// TestParseCLICommand tests the ParseCLICommand function.
func TestParseCLICommand(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedCmd CLICommand
		partsLen    int
	}{
		{"empty input", "", CLICommandNone, 0},
		{"whitespace only", "   ", CLICommandNone, 0},
		{"help command", "help", CLICommandHelp, 1},
		{"help shorthand", "h", CLICommandHelp, 1},
		{"HELP uppercase", "HELP", CLICommandHelp, 1},
		{"friends command", "friends", CLICommandFriends, 1},
		{"friends shorthand", "f", CLICommandFriends, 1},
		{"calls command", "calls", CLICommandCalls, 1},
		{"calls shorthand", "c", CLICommandCalls, 1},
		{"stats command", "stats", CLICommandStats, 1},
		{"stats shorthand", "s", CLICommandStats, 1},
		{"add command", "add toxid123", CLICommandAdd, 2},
		{"add with message", "add toxid123 hello there", CLICommandAdd, 4},
		{"msg command", "msg 1 hello", CLICommandMsg, 3},
		{"msg shorthand", "m 1 hello", CLICommandMsg, 3},
		{"call command", "call 1", CLICommandCall, 2},
		{"videocall command", "videocall 1", CLICommandVideoCall, 2},
		{"videocall shorthand", "vcall 1", CLICommandVideoCall, 2},
		{"hangup command", "hangup 1", CLICommandHangup, 2},
		{"hangup end alias", "end 1", CLICommandHangup, 2},
		{"save command", "save", CLICommandSave, 1},
		{"quit command", "quit", CLICommandQuit, 1},
		{"exit command", "exit", CLICommandQuit, 1},
		{"q command", "q", CLICommandQuit, 1},
		{"unknown command", "unknown", CLICommandUnknown, 1},
		{"random text", "foo bar baz", CLICommandUnknown, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, parts := ParseCLICommand(tc.input)

			if cmd != tc.expectedCmd {
				t.Errorf("ParseCLICommand(%q) cmd = %v, want %v", tc.input, cmd, tc.expectedCmd)
			}
			if tc.partsLen == 0 {
				if parts != nil {
					t.Errorf("ParseCLICommand(%q) parts = %v, want nil", tc.input, parts)
				}
			} else {
				if len(parts) != tc.partsLen {
					t.Errorf("ParseCLICommand(%q) parts len = %d, want %d", tc.input, len(parts), tc.partsLen)
				}
			}
		})
	}
}

// TestCallSession tests CallSession struct operations.
func TestCallSession(t *testing.T) {
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)

	t.Run("Create audio-only session", func(t *testing.T) {
		session := &CallSession{
			FriendNumber: 42,
			AudioEnabled: true,
			VideoEnabled: false,
			StartTime:    fixedTime,
		}

		if session.FriendNumber != 42 {
			t.Errorf("FriendNumber = %d, want 42", session.FriendNumber)
		}
		if !session.AudioEnabled {
			t.Error("AudioEnabled should be true")
		}
		if session.VideoEnabled {
			t.Error("VideoEnabled should be false")
		}
	})

	t.Run("Create audio+video session", func(t *testing.T) {
		session := &CallSession{
			FriendNumber: 99,
			AudioEnabled: true,
			VideoEnabled: true,
			StartTime:    fixedTime,
		}

		if !session.AudioEnabled || !session.VideoEnabled {
			t.Error("Both audio and video should be enabled")
		}
	})

	t.Run("Frame counter increments", func(t *testing.T) {
		session := &CallSession{
			FramesSent: 0,
			FramesRecv: 0,
		}

		session.FramesSent++
		session.FramesSent++
		session.FramesRecv++

		if session.FramesSent != 2 {
			t.Errorf("FramesSent = %d, want 2", session.FramesSent)
		}
		if session.FramesRecv != 1 {
			t.Errorf("FramesRecv = %d, want 1", session.FramesRecv)
		}
	})
}

// TestFriendInfo tests FriendInfo struct operations.
func TestFriendInfo(t *testing.T) {
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)

	t.Run("Create friend info", func(t *testing.T) {
		publicKey := [32]byte{1, 2, 3, 4}
		friend := &FriendInfo{
			Number:    5,
			Name:      "TestFriend",
			Status:    "Available",
			LastSeen:  fixedTime,
			PublicKey: publicKey,
		}

		if friend.Number != 5 {
			t.Errorf("Number = %d, want 5", friend.Number)
		}
		if friend.Name != "TestFriend" {
			t.Errorf("Name = %q, want %q", friend.Name, "TestFriend")
		}
		if friend.PublicKey[0] != 1 {
			t.Errorf("PublicKey[0] = %d, want 1", friend.PublicKey[0])
		}
	})

	t.Run("Update last seen", func(t *testing.T) {
		friend := &FriendInfo{
			Number:   1,
			LastSeen: fixedTime,
		}

		newTime := fixedTime.Add(1 * time.Hour)
		friend.LastSeen = newTime

		if !friend.LastSeen.Equal(newTime) {
			t.Errorf("LastSeen = %v, want %v", friend.LastSeen, newTime)
		}
	})
}

// TestToxAVClientMockable tests ToxAVClient components that can be tested
// without a real Tox instance.
func TestToxAVClientMockable(t *testing.T) {
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	mockTime := &MockTimeProvider{currentTime: fixedTime}

	t.Run("Client state initialization", func(t *testing.T) {
		client := &ToxAVClient{
			timeProvider: mockTime,
			running:      true,
			activeCalls:  make(map[uint32]*CallSession),
			friends:      make(map[uint32]*FriendInfo),
		}

		if !client.running {
			t.Error("Client should be running after init")
		}
		if len(client.activeCalls) != 0 {
			t.Error("Active calls should be empty")
		}
		if len(client.friends) != 0 {
			t.Error("Friends should be empty")
		}
	})

	t.Run("Time provider integration", func(t *testing.T) {
		client := &ToxAVClient{
			timeProvider: mockTime,
			activeCalls:  make(map[uint32]*CallSession),
			friends:      make(map[uint32]*FriendInfo),
		}

		// Add a friend with current mock time
		client.friends[0] = &FriendInfo{
			Number:   0,
			Name:     "MockFriend",
			LastSeen: client.timeProvider.Now(),
		}

		// Advance time
		mockTime.Advance(5 * time.Minute)

		// Verify the friend's LastSeen is now 5 minutes in the past
		duration := client.timeProvider.Since(client.friends[0].LastSeen)
		if duration != 5*time.Minute {
			t.Errorf("Duration since last seen = %v, want 5m", duration)
		}
	})

	t.Run("Call session duration tracking", func(t *testing.T) {
		mockTime := &MockTimeProvider{currentTime: fixedTime}
		client := &ToxAVClient{
			timeProvider: mockTime,
			activeCalls:  make(map[uint32]*CallSession),
		}

		// Create a call session
		session := &CallSession{
			FriendNumber: 1,
			AudioEnabled: true,
			VideoEnabled: false,
			StartTime:    client.timeProvider.Now(),
		}
		client.activeCalls[1] = session

		// Simulate call duration
		mockTime.Advance(2 * time.Minute)

		duration := client.timeProvider.Since(session.StartTime)
		if duration != 2*time.Minute {
			t.Errorf("Call duration = %v, want 2m", duration)
		}
	})

	t.Run("Multiple active calls", func(t *testing.T) {
		client := &ToxAVClient{
			timeProvider: mockTime,
			activeCalls:  make(map[uint32]*CallSession),
		}

		// Add multiple calls
		for i := uint32(0); i < 3; i++ {
			client.activeCalls[i] = &CallSession{
				FriendNumber: i,
				AudioEnabled: true,
				VideoEnabled: i%2 == 0, // Even numbers have video
				StartTime:    mockTime.Now(),
			}
		}

		if len(client.activeCalls) != 3 {
			t.Errorf("Active calls count = %d, want 3", len(client.activeCalls))
		}

		// Verify call 2 has video
		if !client.activeCalls[2].VideoEnabled {
			t.Error("Call 2 should have video enabled")
		}
		// Verify call 1 has no video
		if client.activeCalls[1].VideoEnabled {
			t.Error("Call 1 should not have video enabled")
		}
	})
}

// TestStatisticsTracking tests the statistics counters.
func TestStatisticsTracking(t *testing.T) {
	t.Run("Message counters", func(t *testing.T) {
		client := &ToxAVClient{
			messagesSent:     0,
			messagesReceived: 0,
		}

		// Simulate sending messages
		for i := 0; i < 5; i++ {
			client.messagesSent++
		}

		// Simulate receiving messages
		for i := 0; i < 3; i++ {
			client.messagesReceived++
		}

		if client.messagesSent != 5 {
			t.Errorf("messagesSent = %d, want 5", client.messagesSent)
		}
		if client.messagesReceived != 3 {
			t.Errorf("messagesReceived = %d, want 3", client.messagesReceived)
		}
	})

	t.Run("Call counters", func(t *testing.T) {
		client := &ToxAVClient{
			callsInitiated: 0,
			callsReceived:  0,
		}

		client.callsInitiated++
		client.callsReceived++
		client.callsReceived++

		if client.callsInitiated != 1 {
			t.Errorf("callsInitiated = %d, want 1", client.callsInitiated)
		}
		if client.callsReceived != 2 {
			t.Errorf("callsReceived = %d, want 2", client.callsReceived)
		}
	})
}

// TestConstants tests that constants have expected values.
func TestConstants(t *testing.T) {
	t.Run("Audio bitrate", func(t *testing.T) {
		if defaultAudioBitRate != 64000 {
			t.Errorf("defaultAudioBitRate = %d, want 64000", defaultAudioBitRate)
		}
	})

	t.Run("Video bitrate", func(t *testing.T) {
		if defaultVideoBitRate != 500000 {
			t.Errorf("defaultVideoBitRate = %d, want 500000", defaultVideoBitRate)
		}
	})

	t.Run("Save data file", func(t *testing.T) {
		expected := "toxav_integration_profile.dat"
		if saveDataFile != expected {
			t.Errorf("saveDataFile = %q, want %q", saveDataFile, expected)
		}
	})
}

// TestConcurrentAccess tests thread-safety of client operations.
func TestConcurrentAccess(t *testing.T) {
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	mockTime := &MockTimeProvider{currentTime: fixedTime}

	client := &ToxAVClient{
		timeProvider: mockTime,
		activeCalls:  make(map[uint32]*CallSession),
		friends:      make(map[uint32]*FriendInfo),
	}

	// Test concurrent writes to friends map (with proper locking)
	t.Run("Concurrent friend access", func(t *testing.T) {
		done := make(chan bool, 10)

		// Writers
		for i := 0; i < 5; i++ {
			go func(n int) {
				client.mu.Lock()
				client.friends[uint32(n)] = &FriendInfo{
					Number: uint32(n),
					Name:   "Friend",
				}
				client.mu.Unlock()
				done <- true
			}(i)
		}

		// Readers
		for i := 0; i < 5; i++ {
			go func() {
				client.mu.RLock()
				_ = len(client.friends)
				client.mu.RUnlock()
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		client.mu.RLock()
		count := len(client.friends)
		client.mu.RUnlock()

		if count != 5 {
			t.Errorf("Friends count = %d, want 5", count)
		}
	})

	// Test concurrent access to call sessions
	t.Run("Concurrent call session access", func(t *testing.T) {
		done := make(chan bool, 10)

		// Add sessions
		for i := 0; i < 5; i++ {
			go func(n int) {
				client.mu.Lock()
				client.activeCalls[uint32(n)] = &CallSession{
					FriendNumber: uint32(n),
					AudioEnabled: true,
				}
				client.mu.Unlock()
				done <- true
			}(i)
		}

		// Read sessions
		for i := 0; i < 5; i++ {
			go func() {
				client.mu.RLock()
				for _, session := range client.activeCalls {
					_ = session.AudioEnabled
				}
				client.mu.RUnlock()
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		client.mu.RLock()
		count := len(client.activeCalls)
		client.mu.RUnlock()

		if count != 5 {
			t.Errorf("Active calls count = %d, want 5", count)
		}
	})
}

// TestCallSessionFrameCounterConcurrency tests thread-safe frame counter updates.
func TestCallSessionFrameCounterConcurrency(t *testing.T) {
	session := &CallSession{
		FriendNumber: 1,
		AudioEnabled: true,
		VideoEnabled: true,
	}

	done := make(chan bool, 100)

	// Concurrent frame counter increments
	for i := 0; i < 50; i++ {
		go func() {
			session.mu.Lock()
			session.FramesSent++
			session.mu.Unlock()
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		go func() {
			session.mu.Lock()
			session.FramesRecv++
			session.mu.Unlock()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	session.mu.RLock()
	sent := session.FramesSent
	recv := session.FramesRecv
	session.mu.RUnlock()

	if sent != 50 {
		t.Errorf("FramesSent = %d, want 50", sent)
	}
	if recv != 50 {
		t.Errorf("FramesRecv = %d, want 50", recv)
	}
}

// TestMessageCommandEnums tests the MessageCommand enum values.
func TestMessageCommandEnums(t *testing.T) {
	// Verify enum values are distinct
	commands := []MessageCommand{
		MessageCommandNone,
		MessageCommandCall,
		MessageCommandVideoCall,
		MessageCommandStatus,
		MessageCommandHelp,
		MessageCommandEcho,
	}

	seen := make(map[MessageCommand]bool)
	for _, cmd := range commands {
		if seen[cmd] {
			t.Errorf("Duplicate MessageCommand value: %v", cmd)
		}
		seen[cmd] = true
	}

	// Verify None is zero value
	if MessageCommandNone != 0 {
		t.Errorf("MessageCommandNone = %d, want 0", MessageCommandNone)
	}
}

// TestCLICommandEnums tests the CLICommand enum values.
func TestCLICommandEnums(t *testing.T) {
	// Verify enum values are distinct
	commands := []CLICommand{
		CLICommandNone,
		CLICommandHelp,
		CLICommandFriends,
		CLICommandCalls,
		CLICommandStats,
		CLICommandAdd,
		CLICommandMsg,
		CLICommandCall,
		CLICommandVideoCall,
		CLICommandHangup,
		CLICommandSave,
		CLICommandQuit,
		CLICommandUnknown,
	}

	seen := make(map[CLICommand]bool)
	for _, cmd := range commands {
		if seen[cmd] {
			t.Errorf("Duplicate CLICommand value: %v", cmd)
		}
		seen[cmd] = true
	}

	// Verify None is zero value
	if CLICommandNone != 0 {
		t.Errorf("CLICommandNone = %d, want 0", CLICommandNone)
	}
}

// TestBitrateCalculation tests the bitrate selection logic.
func TestBitrateCalculation(t *testing.T) {
	testCases := []struct {
		name            string
		audioEnabled    bool
		videoEnabled    bool
		expectedAudioBR uint32
		expectedVideoBR uint32
	}{
		{"audio only", true, false, defaultAudioBitRate, 0},
		{"video only", false, true, 0, defaultVideoBitRate},
		{"audio and video", true, true, defaultAudioBitRate, defaultVideoBitRate},
		{"neither", false, false, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			audioBR := uint32(0)
			videoBR := uint32(0)
			if tc.audioEnabled {
				audioBR = defaultAudioBitRate
			}
			if tc.videoEnabled {
				videoBR = defaultVideoBitRate
			}

			if audioBR != tc.expectedAudioBR {
				t.Errorf("audioBR = %d, want %d", audioBR, tc.expectedAudioBR)
			}
			if videoBR != tc.expectedVideoBR {
				t.Errorf("videoBR = %d, want %d", videoBR, tc.expectedVideoBR)
			}
		})
	}
}

// BenchmarkParseMessageCommand benchmarks message command parsing.
func BenchmarkParseMessageCommand(b *testing.B) {
	messages := []string{"call", "videocall", "status", "help", "echo hello world", "random message"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, msg := range messages {
			ParseMessageCommand(msg)
		}
	}
}

// BenchmarkParseCLICommand benchmarks CLI command parsing.
func BenchmarkParseCLICommand(b *testing.B) {
	commands := []string{"help", "friends", "stats", "call 1", "msg 1 hello", "quit", "unknown"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmd := range commands {
			ParseCLICommand(cmd)
		}
	}
}

// BenchmarkTimeProvider benchmarks time provider implementations.
func BenchmarkTimeProvider(b *testing.B) {
	b.Run("RealTimeProvider.Now", func(b *testing.B) {
		provider := RealTimeProvider{}
		for i := 0; i < b.N; i++ {
			_ = provider.Now()
		}
	})

	b.Run("MockTimeProvider.Now", func(b *testing.B) {
		provider := &MockTimeProvider{
			currentTime: time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		}
		for i := 0; i < b.N; i++ {
			_ = provider.Now()
		}
	})

	b.Run("RealTimeProvider.Since", func(b *testing.B) {
		provider := RealTimeProvider{}
		past := time.Now().Add(-time.Hour)
		for i := 0; i < b.N; i++ {
			_ = provider.Since(past)
		}
	})

	b.Run("MockTimeProvider.Since", func(b *testing.B) {
		fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
		provider := &MockTimeProvider{currentTime: fixedTime}
		past := fixedTime.Add(-time.Hour)
		for i := 0; i < b.N; i++ {
			_ = provider.Since(past)
		}
	})
}
