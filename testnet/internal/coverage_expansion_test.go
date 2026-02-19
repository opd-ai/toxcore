package internal

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestBootstrapServerGetStatus tests the GetStatus method with a partially initialized server.
func TestBootstrapServerGetStatus(t *testing.T) {
	mockTime := NewMockTimeProvider(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))

	// Create a minimal server struct to test GetStatus logic
	server := &BootstrapServer{
		address:      "127.0.0.1",
		port:         33445,
		running:      true,
		publicKey:    [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		timeProvider: mockTime,
		metrics: &ServerMetrics{
			StartTime:         time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			ConnectionsServed: 100,
			PacketsProcessed:  5000,
			ActiveClients:     3,
		},
	}

	// Note: GetStatus calls tox.SelfGetConnectionStatus() which requires a real tox instance
	// We can't fully test GetStatus without a mock, but we can verify fields are accessible

	// Test GetAddress
	if server.GetAddress() != "127.0.0.1" {
		t.Errorf("GetAddress() = %s, want %s", server.GetAddress(), "127.0.0.1")
	}

	// Test GetPort
	if server.GetPort() != 33445 {
		t.Errorf("GetPort() = %d, want %d", server.GetPort(), 33445)
	}

	// Test GetPublicKeyHex
	pubKeyHex := server.GetPublicKeyHex()
	expectedHex := "0102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F20"
	if pubKeyHex != expectedHex {
		t.Errorf("GetPublicKeyHex() = %s, want %s", pubKeyHex, expectedHex)
	}

	// Test GetPublicKey
	pubKey := server.GetPublicKey()
	if pubKey[0] != 1 || pubKey[31] != 32 {
		t.Error("GetPublicKey() returned unexpected value")
	}

	// Test IsRunning
	if !server.IsRunning() {
		t.Error("IsRunning() should return true")
	}

	// Test GetMetrics
	metrics := server.GetMetrics()
	if metrics.ConnectionsServed != 100 {
		t.Errorf("GetMetrics().ConnectionsServed = %d, want %d", metrics.ConnectionsServed, 100)
	}
	if metrics.PacketsProcessed != 5000 {
		t.Errorf("GetMetrics().PacketsProcessed = %d, want %d", metrics.PacketsProcessed, 5000)
	}
	if metrics.ActiveClients != 3 {
		t.Errorf("GetMetrics().ActiveClients = %d, want %d", metrics.ActiveClients, 3)
	}

	// Test SetTimeProvider
	newMockTime := NewMockTimeProvider(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	server.SetTimeProvider(newMockTime)
	if server.getTimeProvider() != newMockTime {
		t.Error("SetTimeProvider did not update time provider")
	}

	// Test SetTimeProvider with nil (should fall back to default)
	server.SetTimeProvider(nil)
	tp := server.getTimeProvider()
	if tp == nil {
		t.Error("getTimeProvider should return default when nil is set")
	}
}

// TestBootstrapServerIsRunningToggle tests the IsRunning method with running state changes.
func TestBootstrapServerIsRunningToggle(t *testing.T) {
	server := &BootstrapServer{
		running: false,
	}

	if server.IsRunning() {
		t.Error("IsRunning() should return false initially")
	}

	// Simulate start
	server.mu.Lock()
	server.running = true
	server.mu.Unlock()

	if !server.IsRunning() {
		t.Error("IsRunning() should return true after starting")
	}

	// Simulate stop
	server.mu.Lock()
	server.running = false
	server.mu.Unlock()

	if server.IsRunning() {
		t.Error("IsRunning() should return false after stopping")
	}
}

// TestServerMetricsConcurrentAccess tests concurrent access to ServerMetrics.
func TestServerMetricsConcurrentAccess(t *testing.T) {
	metrics := &ServerMetrics{
		StartTime:         time.Now(),
		ConnectionsServed: 0,
		PacketsProcessed:  0,
		ActiveClients:     0,
	}

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			metrics.mu.Lock()
			metrics.ConnectionsServed++
			metrics.PacketsProcessed += 10
			metrics.mu.Unlock()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			metrics.mu.RLock()
			_ = metrics.ConnectionsServed
			_ = metrics.PacketsProcessed
			metrics.mu.RUnlock()
		}
		done <- true
	}()

	<-done
	<-done

	if metrics.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want %d", metrics.ConnectionsServed, 100)
	}
	if metrics.PacketsProcessed != 1000 {
		t.Errorf("PacketsProcessed = %d, want %d", metrics.PacketsProcessed, 1000)
	}
}

// TestTestClientGettersWithoutTox tests TestClient getter methods that don't require Tox.
func TestTestClientGettersWithoutTox(t *testing.T) {
	mockTime := NewMockTimeProvider(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))

	client := &TestClient{
		name:         "TestAlice",
		connected:    true,
		friends:      make(map[uint32]*FriendConnection),
		timeProvider: mockTime,
		metrics: &ClientMetrics{
			StartTime:          time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			MessagesSent:       50,
			MessagesReceived:   45,
			FriendRequestsSent: 5,
			FriendRequestsRecv: 3,
			ConnectionEvents:   10,
		},
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
		logger:          logrus.WithField("test", "client"),
	}

	// Add a friend for testing
	client.friends[1] = &FriendConnection{
		FriendID:     1,
		PublicKey:    [32]byte{1, 2, 3},
		Status:       FriendStatusOnline,
		LastSeen:     mockTime.Now(),
		MessagesSent: 10,
		MessagesRecv: 8,
	}

	// Test GetName
	if client.GetName() != "TestAlice" {
		t.Errorf("GetName() = %s, want %s", client.GetName(), "TestAlice")
	}

	// Test IsConnected
	if !client.IsConnected() {
		t.Error("IsConnected() should return true")
	}

	// Test GetFriends returns a copy
	friends := client.GetFriends()
	if len(friends) != 1 {
		t.Errorf("GetFriends() returned %d friends, want 1", len(friends))
	}

	friend, exists := friends[1]
	if !exists {
		t.Error("Friend with ID 1 should exist")
	}
	if friend.MessagesSent != 10 {
		t.Errorf("Friend.MessagesSent = %d, want %d", friend.MessagesSent, 10)
	}

	// Verify GetFriends returns a copy (modification doesn't affect original)
	friends[1].MessagesSent = 999
	origFriend := client.friends[1]
	if origFriend.MessagesSent == 999 {
		t.Error("GetFriends should return a copy, not the original")
	}

	// Test GetMetrics
	metrics := client.GetMetrics()
	if metrics.MessagesSent != 50 {
		t.Errorf("GetMetrics().MessagesSent = %d, want %d", metrics.MessagesSent, 50)
	}
	if metrics.MessagesReceived != 45 {
		t.Errorf("GetMetrics().MessagesReceived = %d, want %d", metrics.MessagesReceived, 45)
	}
	if metrics.FriendRequestsSent != 5 {
		t.Errorf("GetMetrics().FriendRequestsSent = %d, want %d", metrics.FriendRequestsSent, 5)
	}

	// Test SetTimeProvider
	newMockTime := NewMockTimeProvider(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	client.SetTimeProvider(newMockTime)
	if client.getTimeProvider() != newMockTime {
		t.Error("SetTimeProvider did not update time provider")
	}
}

// TestTestClientIsConnectedToggle tests the IsConnected method with connection state changes.
func TestTestClientIsConnectedToggle(t *testing.T) {
	client := &TestClient{
		connected: false,
	}

	if client.IsConnected() {
		t.Error("IsConnected() should return false initially")
	}

	// Simulate connection
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	if !client.IsConnected() {
		t.Error("IsConnected() should return true after connecting")
	}

	// Simulate disconnection
	client.mu.Lock()
	client.connected = false
	client.mu.Unlock()

	if client.IsConnected() {
		t.Error("IsConnected() should return false after disconnecting")
	}
}

// TestClientMetricsConcurrentAccess tests concurrent access to ClientMetrics.
func TestClientMetricsConcurrentAccess(t *testing.T) {
	metrics := &ClientMetrics{
		StartTime: time.Now(),
	}

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			metrics.mu.Lock()
			metrics.MessagesSent++
			metrics.MessagesReceived++
			metrics.mu.Unlock()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			metrics.mu.RLock()
			_ = metrics.MessagesSent
			_ = metrics.MessagesReceived
			metrics.mu.RUnlock()
		}
		done <- true
	}()

	<-done
	<-done

	if metrics.MessagesSent != 100 {
		t.Errorf("MessagesSent = %d, want %d", metrics.MessagesSent, 100)
	}
	if metrics.MessagesReceived != 100 {
		t.Errorf("MessagesReceived = %d, want %d", metrics.MessagesReceived, 100)
	}
}

// TestGetFriendsEmptyMap tests GetFriends with an empty friends map.
func TestGetFriendsEmptyMap(t *testing.T) {
	client := &TestClient{
		friends: make(map[uint32]*FriendConnection),
	}

	friends := client.GetFriends()
	if len(friends) != 0 {
		t.Errorf("GetFriends() should return empty map, got %d friends", len(friends))
	}
}

// TestGetFriendsMultipleFriends tests GetFriends with multiple friends.
func TestGetFriendsMultipleFriends(t *testing.T) {
	client := &TestClient{
		friends: make(map[uint32]*FriendConnection),
	}

	// Add multiple friends
	for i := uint32(1); i <= 5; i++ {
		client.friends[i] = &FriendConnection{
			FriendID:     i,
			Status:       FriendStatus(i % 4),
			MessagesSent: int(i * 10),
			MessagesRecv: int(i * 5),
		}
	}

	friends := client.GetFriends()
	if len(friends) != 5 {
		t.Errorf("GetFriends() returned %d friends, want 5", len(friends))
	}

	// Verify each friend is correctly copied
	for i := uint32(1); i <= 5; i++ {
		friend, exists := friends[i]
		if !exists {
			t.Errorf("Friend %d should exist", i)
			continue
		}
		if friend.FriendID != i {
			t.Errorf("Friend %d has wrong FriendID = %d", i, friend.FriendID)
		}
		if friend.MessagesSent != int(i*10) {
			t.Errorf("Friend %d.MessagesSent = %d, want %d", i, friend.MessagesSent, i*10)
		}
	}
}

// TestProtocolTestSuiteGetFriendIDsForMessaging tests the getFriendIDsForMessaging method.
func TestProtocolTestSuiteGetFriendIDsForMessaging(t *testing.T) {
	suite := &ProtocolTestSuite{
		logger: logrus.WithField("test", "friend-ids"),
		config: DefaultProtocolConfig(),
	}

	// Set up clients with friends
	suite.clientA = &TestClient{
		friends: map[uint32]*FriendConnection{
			42: {FriendID: 42},
		},
	}
	suite.clientB = &TestClient{
		friends: map[uint32]*FriendConnection{
			99: {FriendID: 99},
		},
	}

	friendIDA, friendIDB, err := suite.getFriendIDsForMessaging()
	if err != nil {
		t.Errorf("getFriendIDsForMessaging() returned error: %v", err)
	}

	if friendIDA != 42 {
		t.Errorf("friendIDA = %d, want 42", friendIDA)
	}
	if friendIDB != 99 {
		t.Errorf("friendIDB = %d, want 99", friendIDB)
	}
}

// TestProtocolTestSuiteGetFriendIDsEmptyFriends tests getFriendIDsForMessaging with empty friends.
func TestProtocolTestSuiteGetFriendIDsEmptyFriends(t *testing.T) {
	suite := &ProtocolTestSuite{
		logger: logrus.WithField("test", "empty-friends"),
		config: DefaultProtocolConfig(),
	}

	suite.clientA = &TestClient{
		friends: make(map[uint32]*FriendConnection),
	}
	suite.clientB = &TestClient{
		friends: make(map[uint32]*FriendConnection),
	}

	friendIDA, friendIDB, err := suite.getFriendIDsForMessaging()
	if err != nil {
		t.Errorf("getFriendIDsForMessaging() returned error: %v", err)
	}

	// With empty maps, both should be 0 (zero value)
	if friendIDA != 0 {
		t.Errorf("friendIDA = %d, want 0", friendIDA)
	}
	if friendIDB != 0 {
		t.Errorf("friendIDB = %d, want 0", friendIDB)
	}
}

// TestNewProtocolTestSuiteWithNilConfigDefaults tests NewProtocolTestSuite with nil config and verifies defaults.
func TestNewProtocolTestSuiteWithNilConfigDefaults(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	if suite == nil {
		t.Fatal("NewProtocolTestSuite(nil) returned nil")
	}

	// Should use default config
	if suite.config == nil {
		t.Error("suite.config should not be nil")
	}

	// Verify default values
	defaultConfig := DefaultProtocolConfig()
	if suite.config.BootstrapTimeout != defaultConfig.BootstrapTimeout {
		t.Errorf("BootstrapTimeout = %v, want %v", suite.config.BootstrapTimeout, defaultConfig.BootstrapTimeout)
	}
}

// TestWaitForConnectionTimeout tests WaitForConnection timeout behavior.
func TestWaitForConnectionTimeout(t *testing.T) {
	client := &TestClient{
		name:         "TimeoutTest",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       logrus.WithField("test", "timeout"),
	}

	// Use a very short timeout
	err := client.WaitForConnection(50 * time.Millisecond)
	if err == nil {
		t.Error("WaitForConnection should return error on timeout")
	}

	expectedSubstr := "timeout waiting for connection"
	if err != nil && !contains(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestWaitForFriendRequestTimeout tests WaitForFriendRequest timeout behavior.
func TestWaitForFriendRequestTimeout(t *testing.T) {
	client := &TestClient{
		name:            "TimeoutTest",
		friendRequestCh: make(chan FriendRequest, 10),
		timeProvider:    NewDefaultTimeProvider(),
		logger:          logrus.WithField("test", "timeout"),
	}

	// Use a very short timeout
	_, err := client.WaitForFriendRequest(50 * time.Millisecond)
	if err == nil {
		t.Error("WaitForFriendRequest should return error on timeout")
	}

	expectedSubstr := "timeout waiting for friend request"
	if err != nil && !contains(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestWaitForFriendRequestSuccess tests WaitForFriendRequest success path.
func TestWaitForFriendRequestSuccess(t *testing.T) {
	client := &TestClient{
		name:            "SuccessTest",
		friendRequestCh: make(chan FriendRequest, 10),
		timeProvider:    NewDefaultTimeProvider(),
		logger:          logrus.WithField("test", "success"),
	}

	// Send a friend request before waiting
	expectedRequest := FriendRequest{
		PublicKey: [32]byte{1, 2, 3},
		Message:   "Test friend request",
		Timestamp: time.Now(),
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		client.friendRequestCh <- expectedRequest
	}()

	request, err := client.WaitForFriendRequest(1 * time.Second)
	if err != nil {
		t.Errorf("WaitForFriendRequest should not return error: %v", err)
	}

	if request == nil {
		t.Fatal("WaitForFriendRequest should return a request")
	}

	if request.Message != expectedRequest.Message {
		t.Errorf("Request message = %q, want %q", request.Message, expectedRequest.Message)
	}
}

// TestWaitForMessageTimeout tests WaitForMessage timeout behavior.
func TestWaitForMessageTimeout(t *testing.T) {
	client := &TestClient{
		name:         "TimeoutTest",
		messageCh:    make(chan Message, 100),
		timeProvider: NewDefaultTimeProvider(),
		logger:       logrus.WithField("test", "timeout"),
	}

	// Use a very short timeout
	_, err := client.WaitForMessage(50 * time.Millisecond)
	if err == nil {
		t.Error("WaitForMessage should return error on timeout")
	}

	expectedSubstr := "timeout waiting for message"
	if err != nil && !contains(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestWaitForMessageSuccess tests WaitForMessage success path.
func TestWaitForMessageSuccess(t *testing.T) {
	client := &TestClient{
		name:         "SuccessTest",
		messageCh:    make(chan Message, 100),
		timeProvider: NewDefaultTimeProvider(),
		logger:       logrus.WithField("test", "success"),
	}

	// Send a message before waiting
	expectedMsg := Message{
		FriendID:  42,
		Content:   "Hello, World!",
		Timestamp: time.Now(),
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		client.messageCh <- expectedMsg
	}()

	msg, err := client.WaitForMessage(1 * time.Second)
	if err != nil {
		t.Errorf("WaitForMessage should not return error: %v", err)
	}

	if msg == nil {
		t.Fatal("WaitForMessage should return a message")
	}

	if msg.Content != expectedMsg.Content {
		t.Errorf("Message content = %q, want %q", msg.Content, expectedMsg.Content)
	}
	if msg.FriendID != expectedMsg.FriendID {
		t.Errorf("Message FriendID = %d, want %d", msg.FriendID, expectedMsg.FriendID)
	}
}

// TestBootstrapServerStartAlreadyRunning tests Start when server is already running.
func TestBootstrapServerStartAlreadyRunning(t *testing.T) {
	server := &BootstrapServer{
		running:  true,
		address:  "127.0.0.1",
		port:     33445,
		stopChan: make(chan struct{}),
		logger:   logrus.WithField("test", "already-running"),
	}

	err := server.Start(context.Background())
	if err == nil {
		t.Error("Start should return error when already running")
	}

	expectedSubstr := "already running"
	if err != nil && !contains(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestBootstrapServerStopNotRunning tests Stop when server is not running.
func TestBootstrapServerStopNotRunning(t *testing.T) {
	server := &BootstrapServer{
		running:  false,
		stopChan: make(chan struct{}),
		logger:   logrus.WithField("test", "not-running"),
	}

	err := server.Stop()
	if err != nil {
		t.Errorf("Stop should not return error when not running: %v", err)
	}
}

// TestWaitForClientsTimeout tests WaitForClients timeout behavior.
func TestWaitForClientsTimeout(t *testing.T) {
	server := &BootstrapServer{
		running:      true,
		timeProvider: NewDefaultTimeProvider(),
		metrics: &ServerMetrics{
			ActiveClients: 0,
		},
		logger: logrus.WithField("test", "wait-clients"),
	}

	// Use a very short timeout
	err := server.WaitForClients(5, 50*time.Millisecond)
	if err == nil {
		t.Error("WaitForClients should return error on timeout")
	}

	expectedSubstr := "timeout waiting for"
	if err != nil && !contains(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestWaitForClientsSuccess tests WaitForClients success path.
func TestWaitForClientsSuccess(t *testing.T) {
	// Create a logger with an explicit output to avoid any shared state issues
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{}) // Discard output for this test
	entry := logger.WithField("test", "wait-clients-success")

	server := &BootstrapServer{
		running:      true,
		timeProvider: NewDefaultTimeProvider(),
		metrics: &ServerMetrics{
			ActiveClients: 0,
		},
		logger: entry,
	}

	// Simulate clients connecting
	go func() {
		time.Sleep(20 * time.Millisecond)
		server.metrics.mu.Lock()
		server.metrics.ActiveClients = 5
		server.metrics.mu.Unlock()
	}()

	err := server.WaitForClients(5, 1*time.Second)
	if err != nil {
		t.Errorf("WaitForClients should not return error: %v", err)
	}
}

// TestUpdateMetrics tests the updateMetrics method.
func TestUpdateMetrics(t *testing.T) {
	server := &BootstrapServer{
		metrics: &ServerMetrics{
			PacketsProcessed: 0,
		},
	}

	// Call updateMetrics multiple times
	for i := 0; i < 10; i++ {
		server.updateMetrics()
	}

	if server.metrics.PacketsProcessed != 10 {
		t.Errorf("PacketsProcessed = %d, want 10", server.metrics.PacketsProcessed)
	}
}

// TestWaitForConnectionSuccess tests WaitForConnection success path.
func TestWaitForConnectionSuccess(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "wait-connection-success")

	client := &TestClient{
		name:         "SuccessTest",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}

	// Simulate connection happening after a short delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		client.mu.Lock()
		client.connected = true
		client.mu.Unlock()
	}()

	err := client.WaitForConnection(1 * time.Second)
	if err != nil {
		t.Errorf("WaitForConnection should not return error: %v", err)
	}
}

// TestLogFinalMetrics tests the logFinalMetrics method.
func TestLogFinalMetrics(t *testing.T) {
	logger := logrus.New()
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "metrics")

	suite := &ProtocolTestSuite{
		logger: entry,
		config: DefaultProtocolConfig(),
	}

	// Set up mock server and clients with minimal required fields
	suite.server = &BootstrapServer{
		running:      true,
		address:      "127.0.0.1",
		port:         33445,
		timeProvider: NewMockTimeProvider(time.Now()),
		metrics: &ServerMetrics{
			StartTime:         time.Now().Add(-5 * time.Minute),
			PacketsProcessed:  1000,
			ActiveClients:     2,
			ConnectionsServed: 10,
		},
	}

	suite.clientA = &TestClient{
		name:         "Alice",
		connected:    true,
		friends:      make(map[uint32]*FriendConnection),
		timeProvider: NewMockTimeProvider(time.Now()),
		metrics: &ClientMetrics{
			StartTime:          time.Now().Add(-5 * time.Minute),
			MessagesSent:       50,
			MessagesReceived:   45,
			FriendRequestsSent: 5,
			FriendRequestsRecv: 0,
		},
	}
	suite.clientA.friends[1] = &FriendConnection{FriendID: 1}

	suite.clientB = &TestClient{
		name:         "Bob",
		connected:    true,
		friends:      make(map[uint32]*FriendConnection),
		timeProvider: NewMockTimeProvider(time.Now()),
		metrics: &ClientMetrics{
			StartTime:          time.Now().Add(-5 * time.Minute),
			MessagesSent:       45,
			MessagesReceived:   50,
			FriendRequestsSent: 0,
			FriendRequestsRecv: 5,
		},
	}
	suite.clientB.friends[1] = &FriendConnection{FriendID: 1}

	// logFinalMetrics calls GetStatus which requires tox, so we can't fully test it
	// But we can verify the metrics structures work correctly
	serverMetrics := suite.server.GetMetrics()
	if serverMetrics.PacketsProcessed != 1000 {
		t.Errorf("Server metrics not correct")
	}

	clientAMetrics := suite.clientA.GetMetrics()
	if clientAMetrics.MessagesSent != 50 {
		t.Errorf("Client A metrics not correct")
	}

	clientBMetrics := suite.clientB.GetMetrics()
	if clientBMetrics.MessagesReceived != 50 {
		t.Errorf("Client B metrics not correct")
	}
}

// TestClientGetFriendsDeepCopy verifies GetFriends returns a deep copy.
func TestClientGetFriendsDeepCopy(t *testing.T) {
	publicKey := [32]byte{1, 2, 3}
	client := &TestClient{
		friends: map[uint32]*FriendConnection{
			1: {
				FriendID:     1,
				PublicKey:    publicKey,
				Status:       FriendStatusOnline,
				MessagesSent: 10,
			},
		},
	}

	// Get friends copy
	friends1 := client.GetFriends()

	// Modify the copy
	friends1[1].MessagesSent = 999
	friends1[2] = &FriendConnection{FriendID: 2} // Add new friend

	// Get another copy
	friends2 := client.GetFriends()

	// Verify original is unchanged
	if friends2[1].MessagesSent != 10 {
		t.Errorf("Original friend was modified through copy")
	}

	if _, exists := friends2[2]; exists {
		t.Error("Adding to copy affected original")
	}
}

// TestServerMetricsReturnsCopy verifies GetMetrics returns a copy.
func TestServerMetricsReturnsCopy(t *testing.T) {
	server := &BootstrapServer{
		metrics: &ServerMetrics{
			PacketsProcessed:  100,
			ConnectionsServed: 10,
			ActiveClients:     5,
		},
	}

	// Get metrics copy
	metrics1 := server.GetMetrics()

	// Modify the copy (note: this is a value copy, not pointer)
	metrics1.PacketsProcessed = 999

	// Get another copy
	metrics2 := server.GetMetrics()

	// Verify original is unchanged
	if metrics2.PacketsProcessed != 100 {
		t.Errorf("Original metrics was modified through copy")
	}
}

// TestClientMetricsReturnsCopy verifies GetMetrics returns a copy.
func TestClientMetricsReturnsCopy(t *testing.T) {
	client := &TestClient{
		metrics: &ClientMetrics{
			MessagesSent:     100,
			MessagesReceived: 50,
		},
	}

	// Get metrics copy
	metrics1 := client.GetMetrics()

	// Modify the copy
	metrics1.MessagesSent = 999

	// Get another copy
	metrics2 := client.GetMetrics()

	// Verify original is unchanged
	if metrics2.MessagesSent != 100 {
		t.Errorf("Original metrics was modified through copy")
	}
}

// TestOrchestratorCleanupWithNoLogFile tests Cleanup when no log file is set.
func TestOrchestratorCleanupWithNoLogFile(t *testing.T) {
	orch, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	// logFile is nil by default

	err = orch.Cleanup()
	if err != nil {
		t.Errorf("Cleanup should not return error when logFile is nil: %v", err)
	}
}

// TestBootstrapServerTimeProviderWithNil tests SetTimeProvider with nil falls back to default.
func TestBootstrapServerTimeProviderWithNil(t *testing.T) {
	server := &BootstrapServer{
		timeProvider: NewMockTimeProvider(time.Now()),
	}

	// Set to nil
	server.SetTimeProvider(nil)

	// Should use default provider
	tp := server.getTimeProvider()
	if tp == nil {
		t.Error("getTimeProvider should return default when timeProvider is nil")
	}
}

// TestTestClientTimeProviderWithNil tests SetTimeProvider with nil falls back to default.
func TestTestClientTimeProviderWithNil(t *testing.T) {
	client := &TestClient{
		timeProvider: NewMockTimeProvider(time.Now()),
	}

	// Set to nil
	client.SetTimeProvider(nil)

	// Should use default provider
	tp := client.getTimeProvider()
	if tp == nil {
		t.Error("getTimeProvider should return default when timeProvider is nil")
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
