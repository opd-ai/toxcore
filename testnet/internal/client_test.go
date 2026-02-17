package internal

import (
	"testing"
	"time"
)

// TestDefaultClientConfig tests the default client configuration.
func TestDefaultClientConfig(t *testing.T) {
	tests := []struct {
		name          string
		clientName    string
		expectedStart uint16
		expectedEnd   uint16
	}{
		{
			name:          "Alice client",
			clientName:    "Alice",
			expectedStart: 33500,
			expectedEnd:   33599,
		},
		{
			name:          "Bob client",
			clientName:    "Bob",
			expectedStart: 33600,
			expectedEnd:   33699,
		},
		{
			name:          "other client",
			clientName:    "Charlie",
			expectedStart: 33700,
			expectedEnd:   33799,
		},
		{
			name:          "default client",
			clientName:    "TestClient",
			expectedStart: 33700,
			expectedEnd:   33799,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultClientConfig(tt.clientName)

			if config == nil {
				t.Fatal("DefaultClientConfig() returned nil")
			}

			if config.Name != tt.clientName {
				t.Errorf("Name = %q, want %q", config.Name, tt.clientName)
			}

			if config.StartPort != tt.expectedStart {
				t.Errorf("StartPort = %d, want %d", config.StartPort, tt.expectedStart)
			}

			if config.EndPort != tt.expectedEnd {
				t.Errorf("EndPort = %d, want %d", config.EndPort, tt.expectedEnd)
			}

			// Check common default values
			if !config.UDPEnabled {
				t.Error("UDPEnabled should be true")
			}

			if config.IPv6Enabled {
				t.Error("IPv6Enabled should be false for localhost testing")
			}

			if config.LocalDiscovery {
				t.Error("LocalDiscovery should be false")
			}

			if config.Logger == nil {
				t.Error("Logger should not be nil")
			}
		})
	}
}

// TestFriendStatusEnum tests the FriendStatus enum values.
func TestFriendStatusEnum(t *testing.T) {
	tests := []struct {
		name     string
		status   FriendStatus
		expected int
	}{
		{"offline", FriendStatusOffline, 0},
		{"online", FriendStatusOnline, 1},
		{"away", FriendStatusAway, 2},
		{"busy", FriendStatusBusy, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.status) != tt.expected {
				t.Errorf("FriendStatus(%s) = %d, want %d", tt.name, int(tt.status), tt.expected)
			}
		})
	}
}

// TestFriendConnectionStruct tests the FriendConnection struct fields.
func TestFriendConnectionStruct(t *testing.T) {
	publicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	now := time.Now()

	conn := &FriendConnection{
		FriendID:     42,
		PublicKey:    publicKey,
		Status:       FriendStatusOnline,
		LastSeen:     now,
		MessagesSent: 10,
		MessagesRecv: 5,
	}

	if conn.FriendID != 42 {
		t.Errorf("FriendID = %d, want %d", conn.FriendID, 42)
	}

	if conn.PublicKey != publicKey {
		t.Error("PublicKey mismatch")
	}

	if conn.Status != FriendStatusOnline {
		t.Errorf("Status = %d, want %d", conn.Status, FriendStatusOnline)
	}

	if conn.LastSeen != now {
		t.Errorf("LastSeen = %v, want %v", conn.LastSeen, now)
	}

	if conn.MessagesSent != 10 {
		t.Errorf("MessagesSent = %d, want %d", conn.MessagesSent, 10)
	}

	if conn.MessagesRecv != 5 {
		t.Errorf("MessagesRecv = %d, want %d", conn.MessagesRecv, 5)
	}
}

// TestFriendRequestStruct tests the FriendRequest struct fields.
func TestFriendRequestStruct(t *testing.T) {
	publicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	now := time.Now()

	request := FriendRequest{
		PublicKey: publicKey,
		Message:   "Hello, let's be friends!",
		Timestamp: now,
	}

	if request.PublicKey != publicKey {
		t.Error("PublicKey mismatch")
	}

	if request.Message != "Hello, let's be friends!" {
		t.Errorf("Message = %q, want %q", request.Message, "Hello, let's be friends!")
	}

	if request.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", request.Timestamp, now)
	}
}

// TestMessageStruct tests the Message struct fields.
func TestMessageStruct(t *testing.T) {
	now := time.Now()

	msg := Message{
		FriendID:  123,
		Content:   "Hello, world!",
		Timestamp: now,
	}

	if msg.FriendID != 123 {
		t.Errorf("FriendID = %d, want %d", msg.FriendID, 123)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", msg.Content, "Hello, world!")
	}

	if msg.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, now)
	}
}

// TestClientMetricsStruct tests the ClientMetrics struct fields.
func TestClientMetricsStruct(t *testing.T) {
	now := time.Now()

	metrics := &ClientMetrics{
		StartTime:          now,
		MessagesSent:       100,
		MessagesReceived:   75,
		FriendRequestsSent: 10,
		FriendRequestsRecv: 5,
		ConnectionEvents:   3,
	}

	if metrics.StartTime != now {
		t.Errorf("StartTime = %v, want %v", metrics.StartTime, now)
	}

	if metrics.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want %d", metrics.MessagesSent, 100)
	}

	if metrics.MessagesReceived != 75 {
		t.Errorf("MessagesReceived = %d, want %d", metrics.MessagesReceived, 75)
	}

	if metrics.FriendRequestsSent != 10 {
		t.Errorf("FriendRequestsSent = %d, want %d", metrics.FriendRequestsSent, 10)
	}

	if metrics.FriendRequestsRecv != 5 {
		t.Errorf("FriendRequestsRecv = %d, want %d", metrics.FriendRequestsRecv, 5)
	}

	if metrics.ConnectionEvents != 3 {
		t.Errorf("ConnectionEvents = %d, want %d", metrics.ConnectionEvents, 3)
	}
}

// TestClientConfigStruct tests the ClientConfig struct fields.
func TestClientConfigStruct(t *testing.T) {
	config := &ClientConfig{
		Name:           "TestClient",
		UDPEnabled:     true,
		IPv6Enabled:    false,
		LocalDiscovery: false,
		StartPort:      33500,
		EndPort:        33599,
	}

	if config.Name != "TestClient" {
		t.Errorf("Name = %q, want %q", config.Name, "TestClient")
	}

	if !config.UDPEnabled {
		t.Error("UDPEnabled should be true")
	}

	if config.IPv6Enabled {
		t.Error("IPv6Enabled should be false")
	}

	if config.LocalDiscovery {
		t.Error("LocalDiscovery should be false")
	}

	if config.StartPort != 33500 {
		t.Errorf("StartPort = %d, want %d", config.StartPort, 33500)
	}

	if config.EndPort != 33599 {
		t.Errorf("EndPort = %d, want %d", config.EndPort, 33599)
	}
}
