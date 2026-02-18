package internal

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestRetryOperationSuccess tests the retry operation when it succeeds on first attempt.
func TestRetryOperationSuccess(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	callCount := 0
	err := suite.retryOperation(func() error {
		callCount++
		return nil // Success on first attempt
	})
	if err != nil {
		t.Errorf("retryOperation should not return error on success: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Operation should be called exactly once on success, called %d times", callCount)
	}
}

// TestRetryOperationFailureAllAttempts tests the retry operation when all attempts fail.
func TestRetryOperationFailureAllAttempts(t *testing.T) {
	config := &ProtocolConfig{
		BootstrapTimeout:     10 * time.Second,
		ConnectionTimeout:    30 * time.Second,
		FriendRequestTimeout: 15 * time.Second,
		MessageTimeout:       10 * time.Second,
		RetryAttempts:        3,
		RetryBackoff:         1 * time.Millisecond, // Very short for testing
		Logger:               logrus.WithField("test", "retry"),
	}
	suite := NewProtocolTestSuite(config)

	callCount := 0
	testErr := errors.New("test error")
	err := suite.retryOperation(func() error {
		callCount++
		return testErr
	})

	if err == nil {
		t.Error("retryOperation should return error when all attempts fail")
	}

	if callCount != 3 {
		t.Errorf("Operation should be called %d times, called %d times", config.RetryAttempts, callCount)
	}
}

// TestRetryOperationSuccessAfterRetries tests the retry operation succeeding after some failures.
func TestRetryOperationSuccessAfterRetries(t *testing.T) {
	config := &ProtocolConfig{
		BootstrapTimeout:     10 * time.Second,
		ConnectionTimeout:    30 * time.Second,
		FriendRequestTimeout: 15 * time.Second,
		MessageTimeout:       10 * time.Second,
		RetryAttempts:        5,
		RetryBackoff:         1 * time.Millisecond, // Very short for testing
		Logger:               logrus.WithField("test", "retry-success"),
	}
	suite := NewProtocolTestSuite(config)

	callCount := 0
	testErr := errors.New("test error")
	err := suite.retryOperation(func() error {
		callCount++
		if callCount < 3 {
			return testErr // Fail first 2 attempts
		}
		return nil // Succeed on 3rd attempt
	})
	if err != nil {
		t.Errorf("retryOperation should succeed after retries: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Operation should be called 3 times, called %d times", callCount)
	}
}

// TestConnectionEventStruct tests the ConnectionEvent struct fields.
func TestConnectionEventStruct(t *testing.T) {
	now := time.Now()
	event := ConnectionEvent{
		Status:    1, // Using numeric value to avoid import
		Timestamp: now,
	}

	if event.Status != 1 {
		t.Errorf("Status = %d, want %d", event.Status, 1)
	}

	if event.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

// TestTestConfigStruct tests the TestConfig struct with all fields set.
func TestTestConfigStruct(t *testing.T) {
	config := &TestConfig{
		BootstrapPort:        44556,
		BootstrapAddress:     "192.168.1.100",
		OverallTimeout:       10 * time.Minute,
		BootstrapTimeout:     20 * time.Second,
		ConnectionTimeout:    60 * time.Second,
		FriendRequestTimeout: 30 * time.Second,
		MessageTimeout:       20 * time.Second,
		RetryAttempts:        5,
		RetryBackoff:         2 * time.Second,
		LogLevel:             "DEBUG",
		LogFile:              "/tmp/test.log",
		VerboseOutput:        true,
		EnableHealthChecks:   true,
		CollectMetrics:       true,
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"BootstrapPort", config.BootstrapPort, uint16(44556)},
		{"BootstrapAddress", config.BootstrapAddress, "192.168.1.100"},
		{"OverallTimeout", config.OverallTimeout, 10 * time.Minute},
		{"BootstrapTimeout", config.BootstrapTimeout, 20 * time.Second},
		{"ConnectionTimeout", config.ConnectionTimeout, 60 * time.Second},
		{"FriendRequestTimeout", config.FriendRequestTimeout, 30 * time.Second},
		{"MessageTimeout", config.MessageTimeout, 20 * time.Second},
		{"RetryAttempts", config.RetryAttempts, 5},
		{"RetryBackoff", config.RetryBackoff, 2 * time.Second},
		{"LogLevel", config.LogLevel, "DEBUG"},
		{"LogFile", config.LogFile, "/tmp/test.log"},
		{"VerboseOutput", config.VerboseOutput, true},
		{"EnableHealthChecks", config.EnableHealthChecks, true},
		{"CollectMetrics", config.CollectMetrics, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestServerMetricsConcurrency tests that ServerMetrics is safe for concurrent access.
func TestServerMetricsConcurrency(t *testing.T) {
	metrics := &ServerMetrics{
		StartTime:         time.Now(),
		ConnectionsServed: 0,
		PacketsProcessed:  0,
		ActiveClients:     0,
	}

	var wg sync.WaitGroup
	iterations := 100

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.mu.Lock()
				metrics.ConnectionsServed++
				metrics.PacketsProcessed++
				metrics.ActiveClients++
				metrics.mu.Unlock()
			}
		}()
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.mu.RLock()
				_ = metrics.ConnectionsServed
				_ = metrics.PacketsProcessed
				_ = metrics.ActiveClients
				metrics.mu.RUnlock()
			}
		}()
	}

	wg.Wait()

	// Verify final counts
	expectedCount := int64(10 * iterations)
	if metrics.ConnectionsServed != expectedCount {
		t.Errorf("ConnectionsServed = %d, want %d", metrics.ConnectionsServed, expectedCount)
	}

	if metrics.PacketsProcessed != expectedCount {
		t.Errorf("PacketsProcessed = %d, want %d", metrics.PacketsProcessed, expectedCount)
	}

	if metrics.ActiveClients != int(expectedCount) {
		t.Errorf("ActiveClients = %d, want %d", metrics.ActiveClients, expectedCount)
	}
}

// TestClientMetricsConcurrency tests that ClientMetrics is safe for concurrent access.
func TestClientMetricsConcurrency(t *testing.T) {
	metrics := &ClientMetrics{
		StartTime:          time.Now(),
		MessagesSent:       0,
		MessagesReceived:   0,
		FriendRequestsSent: 0,
		FriendRequestsRecv: 0,
		ConnectionEvents:   0,
	}

	var wg sync.WaitGroup
	iterations := 100

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.mu.Lock()
				metrics.MessagesSent++
				metrics.MessagesReceived++
				metrics.FriendRequestsSent++
				metrics.FriendRequestsRecv++
				metrics.ConnectionEvents++
				metrics.mu.Unlock()
			}
		}()
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.mu.RLock()
				_ = metrics.MessagesSent
				_ = metrics.MessagesReceived
				_ = metrics.FriendRequestsSent
				_ = metrics.FriendRequestsRecv
				_ = metrics.ConnectionEvents
				metrics.mu.RUnlock()
			}
		}()
	}

	wg.Wait()

	// Verify final counts
	expectedCount := int64(10 * iterations)
	if metrics.MessagesSent != expectedCount {
		t.Errorf("MessagesSent = %d, want %d", metrics.MessagesSent, expectedCount)
	}

	if metrics.MessagesReceived != expectedCount {
		t.Errorf("MessagesReceived = %d, want %d", metrics.MessagesReceived, expectedCount)
	}
}

// TestOrchestratorExecuteWithStepTracking tests step tracking functionality.
func TestOrchestratorExecuteWithStepTracking(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Test successful step
	stepName := "Test Step Success"
	err = orchestrator.executeWithStepTracking(stepName, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("executeWithStepTracking should not return error for successful operation: %v", err)
	}

	// Verify step was recorded
	results := orchestrator.GetResults()
	if len(results.TestSteps) != 1 {
		t.Fatalf("Expected 1 test step, got %d", len(results.TestSteps))
	}

	step := results.TestSteps[0]
	if step.StepName != stepName {
		t.Errorf("StepName = %q, want %q", step.StepName, stepName)
	}

	if step.Status != TestStatusPassed {
		t.Errorf("Status = %s, want %s", step.Status, TestStatusPassed)
	}

	if step.ExecutionTime <= 0 {
		t.Error("ExecutionTime should be positive")
	}
}

// TestOrchestratorExecuteWithStepTrackingFailure tests step tracking with failure.
func TestOrchestratorExecuteWithStepTrackingFailure(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Test failed step
	stepName := "Test Step Failure"
	testErr := errors.New("intentional test failure")
	err = orchestrator.executeWithStepTracking(stepName, func() error {
		return testErr
	})

	if err == nil {
		t.Error("executeWithStepTracking should return error for failed operation")
	}

	// Verify step was recorded with failure status
	results := orchestrator.GetResults()
	if len(results.TestSteps) != 1 {
		t.Fatalf("Expected 1 test step, got %d", len(results.TestSteps))
	}

	step := results.TestSteps[0]
	if step.Status != TestStatusFailed {
		t.Errorf("Status = %s, want %s", step.Status, TestStatusFailed)
	}

	if step.ErrorMessage == "" {
		t.Error("ErrorMessage should be set for failed step")
	}
}

// TestOrchestratorTimeProviderSetting tests the SetTimeProvider functionality.
func TestOrchestratorTimeProviderSetting(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Create mock time provider
	fixedTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mockTP := NewMockTimeProvider(fixedTime)

	// Set mock time provider
	orchestrator.SetTimeProvider(mockTP)

	// Verify the time provider is used
	if orchestrator.getTimeProvider().Now() != fixedTime {
		t.Errorf("TimeProvider.Now() = %v, want %v", orchestrator.getTimeProvider().Now(), fixedTime)
	}
}

// TestBootstrapServerTimeProviderSetting tests the SetTimeProvider for BootstrapServer.
func TestBootstrapServerTimeProviderSetting(t *testing.T) {
	server := &BootstrapServer{
		stopChan: make(chan struct{}),
	}

	// Create mock time provider
	fixedTime := time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC)
	mockTP := NewMockTimeProvider(fixedTime)

	// Set mock time provider
	server.SetTimeProvider(mockTP)

	// Verify the time provider is used
	if server.getTimeProvider().Now() != fixedTime {
		t.Errorf("TimeProvider.Now() = %v, want %v", server.getTimeProvider().Now(), fixedTime)
	}
}

// TestClientTimeProvider tests the SetTimeProvider for TestClient.
func TestClientTimeProvider(t *testing.T) {
	client := &TestClient{
		name:            "TestClient",
		friends:         make(map[uint32]*FriendConnection),
		metrics:         &ClientMetrics{},
		timeProvider:    NewDefaultTimeProvider(),
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	// Create mock time provider
	fixedTime := time.Date(2026, 3, 20, 14, 45, 0, 0, time.UTC)
	mockTP := NewMockTimeProvider(fixedTime)

	// Set mock time provider
	client.SetTimeProvider(mockTP)

	// Verify the time provider is used
	if client.getTimeProvider().Now() != fixedTime {
		t.Errorf("TimeProvider.Now() = %v, want %v", client.getTimeProvider().Now(), fixedTime)
	}
}

// TestFriendConnectionCopy tests that GetFriends returns a copy of friends map.
func TestFriendConnectionCopy(t *testing.T) {
	publicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	now := time.Now()

	client := &TestClient{
		name: "TestClient",
		friends: map[uint32]*FriendConnection{
			1: {
				FriendID:     1,
				PublicKey:    publicKey,
				Status:       FriendStatusOnline,
				LastSeen:     now,
				MessagesSent: 5,
				MessagesRecv: 3,
			},
		},
		metrics:         &ClientMetrics{},
		timeProvider:    NewDefaultTimeProvider(),
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	// Get friends copy
	friendsCopy := client.GetFriends()

	// Modify the copy
	friendsCopy[1].MessagesSent = 100

	// Verify original is unchanged
	if client.friends[1].MessagesSent == 100 {
		t.Error("Modifying returned friends should not affect original")
	}
}

// TestTestStatusEnum tests all TestStatus enum values.
func TestTestStatusEnum(t *testing.T) {
	tests := []struct {
		status   TestStatus
		expected int
	}{
		{TestStatusPending, 0},
		{TestStatusRunning, 1},
		{TestStatusPassed, 2},
		{TestStatusFailed, 3},
		{TestStatusSkipped, 4},
		{TestStatusTimeout, 5},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if int(tt.status) != tt.expected {
				t.Errorf("TestStatus value = %d, want %d", int(tt.status), tt.expected)
			}
		})
	}
}

// TestMultipleStepTracking tests that multiple steps are tracked correctly.
func TestMultipleStepTracking(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Execute multiple steps
	steps := []string{"Step 1", "Step 2", "Step 3"}
	for _, stepName := range steps {
		_ = orchestrator.executeWithStepTracking(stepName, func() error {
			return nil
		})
	}

	// Verify all steps were recorded
	results := orchestrator.GetResults()
	if len(results.TestSteps) != len(steps) {
		t.Errorf("Expected %d test steps, got %d", len(steps), len(results.TestSteps))
	}

	for i, step := range results.TestSteps {
		if step.StepName != steps[i] {
			t.Errorf("Step %d name = %q, want %q", i, step.StepName, steps[i])
		}
	}
}

// TestProtocolCleanupHelpers tests the cleanup helper functions with nil pointers.
func TestProtocolCleanupHelpers(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	var errors []error

	// Test cleanup helpers with nil components (should not panic)
	suite.cleanupClientA(&errors)
	suite.cleanupClientB(&errors)
	suite.cleanupServer(&errors)

	// No errors should be added for nil components
	if len(errors) != 0 {
		t.Errorf("Expected no errors for nil components, got %d", len(errors))
	}
}

// TestReportCleanupResultsNoErrors tests reportCleanupResults with no errors.
func TestReportCleanupResultsNoErrors(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	var errors []error
	err := suite.reportCleanupResults(errors)
	if err != nil {
		t.Errorf("reportCleanupResults should return nil for empty errors: %v", err)
	}
}

// TestReportCleanupResultsWithErrors tests reportCleanupResults with errors.
func TestReportCleanupResultsWithErrors(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	errors := []error{
		errors.New("error 1"),
		errors.New("error 2"),
	}
	err := suite.reportCleanupResults(errors)

	if err == nil {
		t.Error("reportCleanupResults should return error when errors slice is not empty")
	}
}

// TestClientIsConnectedInitialState tests IsConnected for initial state.
func TestClientIsConnectedInitialState(t *testing.T) {
	client := &TestClient{
		name:            "TestClient",
		connected:       false,
		friends:         make(map[uint32]*FriendConnection),
		metrics:         &ClientMetrics{},
		timeProvider:    NewDefaultTimeProvider(),
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

// TestClientGetNameReturnsCorrectName tests GetName returns correct client name.
func TestClientGetNameReturnsCorrectName(t *testing.T) {
	client := &TestClient{
		name:            "Alice",
		friends:         make(map[uint32]*FriendConnection),
		metrics:         &ClientMetrics{},
		timeProvider:    NewDefaultTimeProvider(),
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	if client.GetName() != "Alice" {
		t.Errorf("GetName() = %q, want %q", client.GetName(), "Alice")
	}
}

// TestBootstrapConfigDefaults tests default bootstrap config values.
func TestBootstrapConfigDefaults(t *testing.T) {
	config := DefaultBootstrapConfig()

	if config.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want %q", config.Address, "127.0.0.1")
	}

	if config.Port != BootstrapDefaultPort {
		t.Errorf("Port = %d, want %d", config.Port, BootstrapDefaultPort)
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}
}

// TestServerGetAddressAndPort tests GetAddress and GetPort methods.
func TestServerGetAddressAndPort(t *testing.T) {
	server := &BootstrapServer{
		address:  "192.168.1.50",
		port:     44557,
		stopChan: make(chan struct{}),
	}

	if server.GetAddress() != "192.168.1.50" {
		t.Errorf("GetAddress() = %q, want %q", server.GetAddress(), "192.168.1.50")
	}

	if server.GetPort() != 44557 {
		t.Errorf("GetPort() = %d, want %d", server.GetPort(), 44557)
	}
}

// TestServerGetPublicKey tests GetPublicKey and GetPublicKeyHex methods.
func TestServerGetPublicKey(t *testing.T) {
	publicKey := [32]byte{
		0xAB, 0xCD, 0xEF, 0x01, 0x23, 0x45, 0x67, 0x89,
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF,
		0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
	}

	server := &BootstrapServer{
		publicKey: publicKey,
		stopChan:  make(chan struct{}),
	}

	if server.GetPublicKey() != publicKey {
		t.Error("GetPublicKey() should return the correct public key")
	}

	expectedHex := "ABCDEF012345678900112233445566778899AABBCCDDEEFF1020304050607080"
	if server.GetPublicKeyHex() != expectedHex {
		t.Errorf("GetPublicKeyHex() = %q, want %q", server.GetPublicKeyHex(), expectedHex)
	}
}

// TestServerIsRunningInitialState tests IsRunning initial state.
func TestServerIsRunningInitialState(t *testing.T) {
	server := &BootstrapServer{
		running:  false,
		stopChan: make(chan struct{}),
	}

	if server.IsRunning() {
		t.Error("Server should not be running initially")
	}
}

// TestClientMetricsGetCopy tests that GetMetrics returns a copy.
func TestClientMetricsGetCopy(t *testing.T) {
	now := time.Now()
	client := &TestClient{
		name:    "TestClient",
		friends: make(map[uint32]*FriendConnection),
		metrics: &ClientMetrics{
			StartTime:          now,
			MessagesSent:       10,
			MessagesReceived:   5,
			FriendRequestsSent: 2,
			FriendRequestsRecv: 1,
			ConnectionEvents:   3,
		},
		timeProvider:    NewDefaultTimeProvider(),
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	metricsCopy := client.GetMetrics()

	// Modify the internal metrics
	client.metrics.mu.Lock()
	client.metrics.MessagesSent = 100
	client.metrics.mu.Unlock()

	// The copy should retain the original value
	if metricsCopy.MessagesSent == 100 {
		t.Error("GetMetrics should return a copy, not a reference")
	}
}

// TestOrchestratorLogConfiguration tests the logConfiguration method.
func TestOrchestratorLogConfiguration(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	config := DefaultTestConfig()
	config.VerboseOutput = true

	orchestrator, err := NewTestOrchestrator(config)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Call logConfiguration
	orchestrator.logConfiguration()

	// Verify log output contains expected fields
	output := buf.String()
	expectedFields := []string{
		"bootstrap_address",
		"bootstrap_port",
		"overall_timeout",
	}

	for _, field := range expectedFields {
		if !bytes.Contains([]byte(output), []byte(field)) {
			t.Errorf("Log output should contain %q", field)
		}
	}
}

// TestOrchestratorLogReportHeader tests the logReportHeader method.
func TestOrchestratorLogReportHeader(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logReportHeader()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Test Execution Summary")) {
		t.Error("Log output should contain 'Test Execution Summary'")
	}
}

// TestOrchestratorLogOverallResults tests the logOverallResults method.
func TestOrchestratorLogOverallResults(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Set some results
	orchestrator.results.TotalTests = 5
	orchestrator.results.PassedTests = 3
	orchestrator.results.FailedTests = 2
	orchestrator.results.FinalStatus = TestStatusFailed
	orchestrator.results.ExecutionTime = 10 * time.Second

	orchestrator.logOverallResults()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Overall Status")) {
		t.Error("Log output should contain 'Overall Status'")
	}
}

// TestOrchestratorLogStepDetailsEmpty tests logStepDetails with no steps.
func TestOrchestratorLogStepDetailsEmpty(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Empty steps
	orchestrator.results.TestSteps = []TestStepResult{}
	orchestrator.logStepDetails()

	// Should not log anything for empty steps
	output := buf.String()
	if bytes.Contains([]byte(output), []byte("Step Details")) {
		t.Error("Log output should not contain 'Step Details' for empty steps")
	}
}

// TestOrchestratorLogStepDetailsWithSteps tests logStepDetails with steps.
func TestOrchestratorLogStepDetailsWithSteps(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Add test steps
	orchestrator.results.TestSteps = []TestStepResult{
		{
			StepName:      "Step 1",
			Status:        TestStatusPassed,
			ExecutionTime: 2 * time.Second,
		},
		{
			StepName:      "Step 2",
			Status:        TestStatusFailed,
			ExecutionTime: 3 * time.Second,
			ErrorMessage:  "test error",
		},
	}

	orchestrator.logStepDetails()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Step Details")) {
		t.Error("Log output should contain 'Step Details'")
	}
}

// TestOrchestratorLogErrorDetailsEmpty tests logErrorDetails with no errors.
func TestOrchestratorLogErrorDetailsEmpty(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// No error details
	orchestrator.results.ErrorDetails = ""
	orchestrator.logErrorDetails()

	// Should not log anything for empty error details
	output := buf.String()
	if bytes.Contains([]byte(output), []byte("Error Details")) {
		t.Error("Log output should not contain 'Error Details' for empty errors")
	}
}

// TestOrchestratorLogErrorDetailsWithErrors tests logErrorDetails with errors.
func TestOrchestratorLogErrorDetailsWithErrors(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Set error details
	orchestrator.results.ErrorDetails = "test error message"
	orchestrator.logErrorDetails()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Error Details")) {
		t.Error("Log output should contain 'Error Details'")
	}
}

// TestOrchestratorLogFinalStatusPassed tests logFinalStatus for passed tests.
func TestOrchestratorLogFinalStatusPassed(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.results.FinalStatus = TestStatusPassed
	orchestrator.logFinalStatus()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("completed successfully")) {
		t.Error("Log output should contain success message for passed tests")
	}
}

// TestOrchestratorLogFinalStatusFailed tests logFinalStatus for failed tests.
func TestOrchestratorLogFinalStatusFailed(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.results.FinalStatus = TestStatusFailed
	orchestrator.logFinalStatus()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("failures")) {
		t.Error("Log output should contain failure message for failed tests")
	}
}

// TestOrchestratorLogSuccessMessage tests logSuccessMessage.
func TestOrchestratorLogSuccessMessage(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logSuccessMessage()

	output := buf.String()
	expectedMessages := []string{
		"All tests completed",
		"PASSED",
		"VERIFIED",
		"CONFIRMED",
	}

	for _, msg := range expectedMessages {
		if !bytes.Contains([]byte(output), []byte(msg)) {
			t.Errorf("Log output should contain %q", msg)
		}
	}
}

// TestOrchestratorLogFailureMessage tests logFailureMessage.
func TestOrchestratorLogFailureMessage(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logFailureMessage()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("failures")) {
		t.Error("Log output should contain 'failures'")
	}
}

// TestOrchestratorLogReportFooter tests logReportFooter.
func TestOrchestratorLogReportFooter(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logReportFooter()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Test run completed")) {
		t.Error("Log output should contain 'Test run completed'")
	}
}

// TestOrchestratorGenerateFinalReport tests generateFinalReport.
func TestOrchestratorGenerateFinalReport(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil)

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Set up results
	orchestrator.results.FinalStatus = TestStatusPassed
	orchestrator.results.TotalTests = 1
	orchestrator.results.PassedTests = 1
	orchestrator.results.ExecutionTime = 5 * time.Second

	orchestrator.generateFinalReport()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Test Execution Summary")) {
		t.Error("Log output should contain 'Test Execution Summary'")
	}
}

// TestServerGetMetrics tests the GetMetrics method for BootstrapServer.
func TestServerGetMetrics(t *testing.T) {
	now := time.Now()
	server := &BootstrapServer{
		stopChan: make(chan struct{}),
		metrics: &ServerMetrics{
			StartTime:         now,
			ConnectionsServed: 100,
			PacketsProcessed:  500,
			ActiveClients:     10,
		},
	}

	metrics := server.GetMetrics()

	if metrics.StartTime != now {
		t.Errorf("StartTime = %v, want %v", metrics.StartTime, now)
	}

	if metrics.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want %d", metrics.ConnectionsServed, 100)
	}

	if metrics.PacketsProcessed != 500 {
		t.Errorf("PacketsProcessed = %d, want %d", metrics.PacketsProcessed, 500)
	}

	if metrics.ActiveClients != 10 {
		t.Errorf("ActiveClients = %d, want %d", metrics.ActiveClients, 10)
	}
}

// TestServerGetMetricsCopy tests that GetMetrics returns a copy.
func TestServerGetMetricsCopy(t *testing.T) {
	now := time.Now()
	server := &BootstrapServer{
		stopChan: make(chan struct{}),
		metrics: &ServerMetrics{
			StartTime:         now,
			ConnectionsServed: 100,
			PacketsProcessed:  500,
			ActiveClients:     10,
		},
	}

	metricsCopy := server.GetMetrics()

	// Modify original
	server.metrics.mu.Lock()
	server.metrics.ConnectionsServed = 200
	server.metrics.mu.Unlock()

	// Copy should not be affected
	if metricsCopy.ConnectionsServed == 200 {
		t.Error("GetMetrics should return a copy, not a reference")
	}
}

// TestServerGetStatus tests the GetStatus method for BootstrapServer.
func TestServerGetStatus(t *testing.T) {
	now := time.Now()
	publicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}

	// Create a minimal mock tox struct - this is tricky without the actual type
	// We'll skip this test as it requires a full toxcore.Tox instance
	t.Skip("Requires full toxcore.Tox instance")

	server := &BootstrapServer{
		running:      true,
		address:      "127.0.0.1",
		port:         33445,
		publicKey:    publicKey,
		stopChan:     make(chan struct{}),
		timeProvider: NewDefaultTimeProvider(),
		metrics: &ServerMetrics{
			StartTime:         now,
			ConnectionsServed: 50,
			PacketsProcessed:  250,
			ActiveClients:     5,
		},
	}

	status := server.GetStatus()

	if status["running"] != true {
		t.Error("status[running] should be true")
	}

	if status["address"] != "127.0.0.1" {
		t.Errorf("status[address] = %v, want %v", status["address"], "127.0.0.1")
	}

	if status["port"] != uint16(33445) {
		t.Errorf("status[port] = %v, want %v", status["port"], uint16(33445))
	}
}

// TestFriendStatusValues tests all FriendStatus enum values.
func TestFriendStatusValues(t *testing.T) {
	tests := []struct {
		status   FriendStatus
		expected int
	}{
		{FriendStatusOffline, 0},
		{FriendStatusOnline, 1},
		{FriendStatusAway, 2},
		{FriendStatusBusy, 3},
	}

	for _, tt := range tests {
		if int(tt.status) != tt.expected {
			t.Errorf("FriendStatus = %d, want %d", int(tt.status), tt.expected)
		}
	}
}

// TestTestResultsCompleteness tests TestResults struct completeness.
func TestTestResultsCompleteness(t *testing.T) {
	results := &TestResults{
		TotalTests:    10,
		PassedTests:   7,
		FailedTests:   2,
		SkippedTests:  1,
		ExecutionTime: 30 * time.Second,
		TestSteps: []TestStepResult{
			{StepName: "Step 1", Status: TestStatusPassed},
			{StepName: "Step 2", Status: TestStatusFailed, ErrorMessage: "error"},
		},
		FinalStatus:  TestStatusFailed,
		ErrorDetails: "2 tests failed",
	}

	// Verify all fields are properly accessible
	if results.TotalTests != results.PassedTests+results.FailedTests+results.SkippedTests {
		t.Error("Test counts should add up to total")
	}

	if len(results.TestSteps) != 2 {
		t.Errorf("Expected 2 test steps, got %d", len(results.TestSteps))
	}

	if results.FinalStatus != TestStatusFailed {
		t.Errorf("FinalStatus = %s, want %s", results.FinalStatus, TestStatusFailed)
	}
}
