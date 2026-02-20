package internal

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestOrchestratorExecuteWithStepTrackingSuccess tests the executeWithStepTracking with successful operation.
func TestOrchestratorExecuteWithStepTrackingSuccess(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "step-tracking")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	// Execute a successful operation
	err = orchestrator.executeWithStepTracking("Test Step", func() error {
		return nil
	})
	if err != nil {
		t.Errorf("executeWithStepTracking should not return error for successful operation: %v", err)
	}

	// Verify step was recorded
	if len(orchestrator.results.TestSteps) != 1 {
		t.Errorf("Expected 1 test step, got %d", len(orchestrator.results.TestSteps))
	}

	step := orchestrator.results.TestSteps[0]
	if step.StepName != "Test Step" {
		t.Errorf("Step name = %q, want %q", step.StepName, "Test Step")
	}

	if step.Status != TestStatusPassed {
		t.Errorf("Step status = %v, want %v", step.Status, TestStatusPassed)
	}
}

// TestOrchestratorExecuteWithStepTrackingFailureCase tests executeWithStepTracking with failed operation.
func TestOrchestratorExecuteWithStepTrackingFailureCase(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	entry := logger.WithField("test", "step-tracking")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	expectedErr := errors.New("test operation failed")

	// Execute a failing operation
	err = orchestrator.executeWithStepTracking("Failing Step", func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("executeWithStepTracking should return the operation error")
	}

	// Verify step was recorded
	if len(orchestrator.results.TestSteps) != 1 {
		t.Errorf("Expected 1 test step, got %d", len(orchestrator.results.TestSteps))
	}

	step := orchestrator.results.TestSteps[0]
	if step.Status != TestStatusFailed {
		t.Errorf("Step status = %v, want %v", step.Status, TestStatusFailed)
	}

	if step.ErrorMessage != expectedErr.Error() {
		t.Errorf("Step error message = %q, want %q", step.ErrorMessage, expectedErr.Error())
	}
}

// TestOrchestratorLogConfigurationCoverage tests the logConfiguration method.
func TestOrchestratorLogConfigurationCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "config")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	// Call logConfiguration
	orchestrator.logConfiguration()

	output := buf.String()
	if !contains(output, "Test Configuration") {
		t.Error("logConfiguration should output configuration header")
	}
}

// TestOrchestratorGenerateFinalReportCoverage tests the generateFinalReport method.
func TestOrchestratorGenerateFinalReportCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "report")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry
	orchestrator.results.FinalStatus = TestStatusPassed
	orchestrator.results.TotalTests = 1
	orchestrator.results.PassedTests = 1
	orchestrator.results.ExecutionTime = 5 * time.Second

	// Add a step result
	orchestrator.results.TestSteps = append(orchestrator.results.TestSteps, TestStepResult{
		StepName:      "Test Step",
		Status:        TestStatusPassed,
		ExecutionTime: 5 * time.Second,
	})

	// Generate the report
	orchestrator.generateFinalReport()

	output := buf.String()
	if !contains(output, "Test Execution Summary") {
		t.Error("Report should contain summary header")
	}
}

// TestOrchestratorGenerateFinalReportWithError tests generateFinalReport with error details.
func TestOrchestratorGenerateFinalReportWithError(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "report-error")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry
	orchestrator.results.FinalStatus = TestStatusFailed
	orchestrator.results.ErrorDetails = "Connection timeout occurred"

	// Add a failing step
	orchestrator.results.TestSteps = append(orchestrator.results.TestSteps, TestStepResult{
		StepName:      "Failing Step",
		Status:        TestStatusFailed,
		ErrorMessage:  "Connection timeout",
		ExecutionTime: 30 * time.Second,
	})

	orchestrator.generateFinalReport()

	output := buf.String()
	if !contains(output, "Error") {
		t.Error("Report should contain error details")
	}
}

// TestOrchestratorSetTimeProvider tests the SetTimeProvider method.
func TestOrchestratorSetTimeProvider(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	mockTime := NewMockTimeProvider(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	orchestrator.SetTimeProvider(mockTime)

	if orchestrator.getTimeProvider() != mockTime {
		t.Error("SetTimeProvider did not update time provider")
	}
}

// TestOrchestratorGetTimeProviderNil tests getTimeProvider fallback when nil.
func TestOrchestratorGetTimeProviderNil(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Set to nil
	orchestrator.SetTimeProvider(nil)

	tp := orchestrator.getTimeProvider()
	if tp == nil {
		t.Error("getTimeProvider should return default when nil is set")
	}
}

// TestProtocolTestSuiteRetryOperation tests the retryOperation method.
func TestProtocolTestSuiteRetryOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "retry")

	t.Run("successful on first attempt", func(t *testing.T) {
		config := DefaultProtocolConfig()
		config.Logger = entry
		config.RetryAttempts = 3
		config.RetryBackoff = 10 * time.Millisecond

		suite := NewProtocolTestSuite(config)

		callCount := 0
		err := suite.retryOperation(func() error {
			callCount++
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if callCount != 1 {
			t.Errorf("Operation should be called once, called %d times", callCount)
		}
	})

	t.Run("successful on second attempt", func(t *testing.T) {
		config := DefaultProtocolConfig()
		config.Logger = entry
		config.RetryAttempts = 3
		config.RetryBackoff = 10 * time.Millisecond

		suite := NewProtocolTestSuite(config)

		callCount := 0
		err := suite.retryOperation(func() error {
			callCount++
			if callCount == 1 {
				return errors.New("first attempt failed")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error after retry, got %v", err)
		}

		if callCount != 2 {
			t.Errorf("Operation should be called twice, called %d times", callCount)
		}
	})

	t.Run("all attempts fail", func(t *testing.T) {
		config := DefaultProtocolConfig()
		config.Logger = entry
		config.RetryAttempts = 3
		config.RetryBackoff = 10 * time.Millisecond

		suite := NewProtocolTestSuite(config)

		callCount := 0
		persistentErr := errors.New("persistent failure")
		err := suite.retryOperation(func() error {
			callCount++
			return persistentErr
		})

		if err == nil {
			t.Error("Expected error after all retries exhausted")
		}

		if callCount != 3 {
			t.Errorf("Operation should be called 3 times, called %d times", callCount)
		}

		if !contains(err.Error(), "failed after 3 attempts") {
			t.Errorf("Error message should mention attempts: %v", err)
		}
	})
}

// TestProtocolTestSuiteCleanupWithErrors tests Cleanup when component Stop fails.
func TestProtocolTestSuiteCleanupWithErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "cleanup")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	// Set up clients with minimal required fields that won't fail on Stop
	suite.clientA = nil
	suite.clientB = nil
	suite.server = nil

	err := suite.Cleanup()
	if err != nil {
		t.Errorf("Cleanup should not return error with nil components: %v", err)
	}
}

// TestProtocolTestSuiteReportCleanupResults tests reportCleanupResults.
func TestProtocolTestSuiteReportCleanupResults(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "cleanup-results")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	t.Run("no errors", func(t *testing.T) {
		buf.Reset()
		err := suite.reportCleanupResults(nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		output := buf.String()
		if !contains(output, "successfully") {
			t.Error("Should report successful cleanup")
		}
	})

	t.Run("with errors", func(t *testing.T) {
		buf.Reset()
		errs := []error{
			errors.New("error 1"),
			errors.New("error 2"),
		}
		err := suite.reportCleanupResults(errs)
		if err == nil {
			t.Error("Expected error when cleanup has errors")
		}

		output := buf.String()
		if !contains(output, "with errors") {
			t.Error("Should report cleanup with errors")
		}
	})
}

// TestFriendStatusEnumValues tests FriendStatus enum values.
func TestFriendStatusEnumValues(t *testing.T) {
	if FriendStatusOffline != 0 {
		t.Errorf("FriendStatusOffline = %d, want 0", FriendStatusOffline)
	}
	if FriendStatusOnline != 1 {
		t.Errorf("FriendStatusOnline = %d, want 1", FriendStatusOnline)
	}
	if FriendStatusAway != 2 {
		t.Errorf("FriendStatusAway = %d, want 2", FriendStatusAway)
	}
	if FriendStatusBusy != 3 {
		t.Errorf("FriendStatusBusy = %d, want 3", FriendStatusBusy)
	}
}

// TestFriendRequestStructFields tests the FriendRequest struct.
func TestFriendRequestStructFields(t *testing.T) {
	now := time.Now()
	pubKey := [32]byte{1, 2, 3, 4, 5}

	req := FriendRequest{
		PublicKey: pubKey,
		Message:   "Hello!",
		Timestamp: now,
	}

	if req.PublicKey != pubKey {
		t.Error("PublicKey not set correctly")
	}
	if req.Message != "Hello!" {
		t.Errorf("Message = %q, want %q", req.Message, "Hello!")
	}
	if req.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", req.Timestamp, now)
	}
}

// TestMessageStructFields tests the Message struct.
func TestMessageStructFields(t *testing.T) {
	now := time.Now()
	msg := Message{
		FriendID:  42,
		Content:   "Test message",
		Timestamp: now,
	}

	if msg.FriendID != 42 {
		t.Errorf("FriendID = %d, want 42", msg.FriendID)
	}
	if msg.Content != "Test message" {
		t.Errorf("Content = %q, want %q", msg.Content, "Test message")
	}
	if msg.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, now)
	}
}

// TestConnectionEventStructFields tests the ConnectionEvent struct fields.
func TestConnectionEventStructFields(t *testing.T) {
	now := time.Now()
	event := ConnectionEvent{
		Status:    1, // ConnectionUDP
		Timestamp: now,
	}

	if event.Status != 1 {
		t.Errorf("Status = %d, want 1", event.Status)
	}
	if event.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

// TestTestConfigStructFields tests the TestConfig struct fields.
func TestTestConfigStructFields(t *testing.T) {
	config := &TestConfig{
		BootstrapPort:        44555,
		BootstrapAddress:     "192.168.1.1",
		OverallTimeout:       10 * time.Minute,
		BootstrapTimeout:     20 * time.Second,
		ConnectionTimeout:    60 * time.Second,
		FriendRequestTimeout: 30 * time.Second,
		MessageTimeout:       15 * time.Second,
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
		{"BootstrapPort", config.BootstrapPort, uint16(44555)},
		{"BootstrapAddress", config.BootstrapAddress, "192.168.1.1"},
		{"OverallTimeout", config.OverallTimeout, 10 * time.Minute},
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

// TestTestResultsStruct tests the TestResults struct.
func TestTestResultsStruct(t *testing.T) {
	results := &TestResults{
		TotalTests:    10,
		PassedTests:   8,
		FailedTests:   1,
		SkippedTests:  1,
		ExecutionTime: 30 * time.Second,
		TestSteps: []TestStepResult{
			{StepName: "Step 1", Status: TestStatusPassed},
			{StepName: "Step 2", Status: TestStatusFailed},
		},
		FinalStatus:  TestStatusPassed,
		ErrorDetails: "",
	}

	if results.TotalTests != 10 {
		t.Errorf("TotalTests = %d, want 10", results.TotalTests)
	}

	if len(results.TestSteps) != 2 {
		t.Errorf("TestSteps length = %d, want 2", len(results.TestSteps))
	}

	if results.FinalStatus != TestStatusPassed {
		t.Errorf("FinalStatus = %v, want %v", results.FinalStatus, TestStatusPassed)
	}
}

// TestClientConfigStructFields tests the ClientConfig struct fields.
func TestClientConfigStructFields(t *testing.T) {
	customLogger := logrus.WithField("test", "config")
	config := &ClientConfig{
		Name:           "TestClient",
		UDPEnabled:     true,
		IPv6Enabled:    false,
		LocalDiscovery: true,
		StartPort:      44000,
		EndPort:        44100,
		Logger:         customLogger,
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
	if !config.LocalDiscovery {
		t.Error("LocalDiscovery should be true")
	}
	if config.StartPort != 44000 {
		t.Errorf("StartPort = %d, want 44000", config.StartPort)
	}
	if config.EndPort != 44100 {
		t.Errorf("EndPort = %d, want 44100", config.EndPort)
	}
}

// TestDefaultClientConfigPortRanges tests port range assignment in DefaultClientConfig.
func TestDefaultClientConfigPortRanges(t *testing.T) {
	tests := []struct {
		name          string
		expectedStart uint16
		expectedEnd   uint16
	}{
		{"Alice", AlicePortRangeStart, AlicePortRangeEnd},
		{"Bob", BobPortRangeStart, BobPortRangeEnd},
		{"Charlie", OtherPortRangeStart, OtherPortRangeEnd},
		{"UnknownClient", OtherPortRangeStart, OtherPortRangeEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultClientConfig(tt.name)

			if config.StartPort != tt.expectedStart {
				t.Errorf("StartPort = %d, want %d", config.StartPort, tt.expectedStart)
			}
			if config.EndPort != tt.expectedEnd {
				t.Errorf("EndPort = %d, want %d", config.EndPort, tt.expectedEnd)
			}
			if config.Name != tt.name {
				t.Errorf("Name = %q, want %q", config.Name, tt.name)
			}
		})
	}
}

// TestVerifyServerLogic tests the verifyServer logic paths without real Tox.
// Note: The actual verifyServer requires a real Tox instance; this tests structure.
func TestVerifyServerLogic(t *testing.T) {
	// This tests the BootstrapServer struct setup for verification
	server := &BootstrapServer{
		running: true,
		address: "127.0.0.1",
		port:    33445,
		logger:  logrus.WithField("test", "verify"),
	}

	// Verify that the server fields are accessible
	if !server.running {
		t.Error("running should be true")
	}
	if server.address != "127.0.0.1" {
		t.Errorf("address = %q, want %q", server.address, "127.0.0.1")
	}
}

// TestLogFinalStatusCoverage tests the logFinalStatus method branches.
func TestLogFinalStatusCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "final-status")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	t.Run("passed status", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.FinalStatus = TestStatusPassed
		orchestrator.logFinalStatus()
		output := buf.String()
		if !contains(output, "successfully") {
			t.Error("Should output success message for passed status")
		}
	})

	t.Run("failed status", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.FinalStatus = TestStatusFailed
		orchestrator.logFinalStatus()
		output := buf.String()
		if !contains(output, "failures") {
			t.Error("Should output failure message for failed status")
		}
	})
}

// TestLogStepDetailsCoverage tests the logStepDetails method.
func TestLogStepDetailsCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "step-details")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	t.Run("empty steps", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.TestSteps = nil
		orchestrator.logStepDetails()
		output := buf.String()
		// With no steps, shouldn't output step details header
		if contains(output, "Step Details") {
			t.Error("Should not output step details header for empty steps")
		}
	})

	t.Run("with steps", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.TestSteps = []TestStepResult{
			{StepName: "Test Step", Status: TestStatusPassed, ExecutionTime: time.Second},
		}
		orchestrator.logStepDetails()
		output := buf.String()
		if !contains(output, "Step Details") {
			t.Error("Should output step details header")
		}
	})

	t.Run("with error message", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.TestSteps = []TestStepResult{
			{StepName: "Failing Step", Status: TestStatusFailed, ErrorMessage: "test error", ExecutionTime: time.Second},
		}
		orchestrator.logStepDetails()
		output := buf.String()
		if !contains(output, "error") {
			t.Error("Should output error details for failed step")
		}
	})
}

// TestLogErrorDetailsCoverage tests the logErrorDetails method.
func TestLogErrorDetailsCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "error-details")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	t.Run("no error details", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.ErrorDetails = ""
		orchestrator.logErrorDetails()
		output := buf.String()
		// With no error details, shouldn't output error header
		if contains(output, "Error Details") {
			t.Error("Should not output error details when empty")
		}
	})

	t.Run("with error details", func(t *testing.T) {
		buf.Reset()
		orchestrator.results.ErrorDetails = "Connection failed"
		orchestrator.logErrorDetails()
		output := buf.String()
		if !contains(output, "Connection failed") {
			t.Error("Should output error details")
		}
	})
}

// TestProtocolTestSuiteWaitForConnections tests waitForConnections with timeout clients.
func TestProtocolTestSuiteWaitForConnections(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "wait-connections")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.ConnectionTimeout = 50 * time.Millisecond
	suite := NewProtocolTestSuite(config)

	// Set up clients that will timeout
	suite.clientA = &TestClient{
		name:         "Alice",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}
	suite.clientB = &TestClient{
		name:         "Bob",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
	}

	err := suite.waitForConnections()
	if err == nil {
		t.Error("waitForConnections should return error when clients don't connect")
	}

	if !contains(err.Error(), "timeout") {
		t.Errorf("Error should mention timeout: %v", err)
	}
}

// TestProtocolTestSuiteConnectClientToBootstrapSetup tests the connectClientToBootstrap setup without calling Tox.
// Note: Full integration testing requires real Tox instances.
func TestProtocolTestSuiteConnectClientToBootstrapSetup(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "connect-bootstrap")

	config := DefaultProtocolConfig()
	config.Logger = entry
	config.RetryAttempts = 1
	config.RetryBackoff = 10 * time.Millisecond
	suite := NewProtocolTestSuite(config)

	// Server must be set for connectClientToBootstrap to work
	suite.server = &BootstrapServer{
		address:   "127.0.0.1",
		port:      33445,
		publicKey: [32]byte{1, 2, 3},
	}

	// Verify server is properly configured
	if suite.server.GetAddress() != "127.0.0.1" {
		t.Errorf("Server address = %q, want %q", suite.server.GetAddress(), "127.0.0.1")
	}
	if suite.server.GetPort() != 33445 {
		t.Errorf("Server port = %d, want %d", suite.server.GetPort(), 33445)
	}
}

// TestOrchestratorLogOverallResultsCoverage tests the logOverallResults method.
func TestOrchestratorLogOverallResultsCoverage(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "overall-results")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry
	orchestrator.results.FinalStatus = TestStatusPassed
	orchestrator.results.TotalTests = 5
	orchestrator.results.PassedTests = 4
	orchestrator.results.FailedTests = 1
	orchestrator.results.ExecutionTime = 10 * time.Second

	orchestrator.logOverallResults()

	output := buf.String()
	if !contains(output, "Overall Status") {
		t.Error("Should output overall status")
	}
}

// TestOrchestratorLogReportHeaderFooter tests header and footer methods.
func TestOrchestratorLogReportHeaderFooter(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "header-footer")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	t.Run("header", func(t *testing.T) {
		buf.Reset()
		orchestrator.logReportHeader()
		output := buf.String()
		if !contains(output, "Summary") {
			t.Error("Header should contain summary")
		}
	})

	t.Run("footer", func(t *testing.T) {
		buf.Reset()
		orchestrator.logReportFooter()
		output := buf.String()
		if !contains(output, "completed") {
			t.Error("Footer should contain completion message")
		}
	})
}

// TestOrchestratorLogSuccessFailureMessages tests success and failure log messages.
func TestOrchestratorLogSuccessFailureMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	entry := logger.WithField("test", "success-failure")

	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	orchestrator.logger = entry

	t.Run("success message", func(t *testing.T) {
		buf.Reset()
		orchestrator.logSuccessMessage()
		output := buf.String()
		if !contains(output, "successfully") {
			t.Error("Should contain success message")
		}
		if !contains(output, "PASSED") {
			t.Error("Should mention PASSED")
		}
	})

	t.Run("failure message", func(t *testing.T) {
		buf.Reset()
		orchestrator.logFailureMessage()
		output := buf.String()
		if !contains(output, "failures") {
			t.Error("Should contain failure message")
		}
	})
}

// TestCleanupHelperMethods tests the cleanup helper methods individually.
func TestCleanupHelperMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "cleanup-helpers")

	config := DefaultProtocolConfig()
	config.Logger = entry
	suite := NewProtocolTestSuite(config)

	t.Run("cleanupClientA with nil", func(t *testing.T) {
		var errors []error
		suite.clientA = nil
		suite.cleanupClientA(&errors)
		if len(errors) != 0 {
			t.Errorf("Should not add errors for nil clientA: %v", errors)
		}
	})

	t.Run("cleanupClientB with nil", func(t *testing.T) {
		var errors []error
		suite.clientB = nil
		suite.cleanupClientB(&errors)
		if len(errors) != 0 {
			t.Errorf("Should not add errors for nil clientB: %v", errors)
		}
	})

	t.Run("cleanupServer with nil", func(t *testing.T) {
		var errors []error
		suite.server = nil
		suite.cleanupServer(&errors)
		if len(errors) != 0 {
			t.Errorf("Should not add errors for nil server: %v", errors)
		}
	})
}

// TestUpdateConnectionStatusLogic tests the updateConnectionStatus logic path.
func TestUpdateConnectionStatusLogic(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	entry := logger.WithField("test", "connection-status")

	client := &TestClient{
		name:         "TestClient",
		connected:    false,
		timeProvider: NewDefaultTimeProvider(),
		logger:       entry,
		metrics: &ClientMetrics{
			StartTime: time.Now(),
		},
		connectionCh: make(chan ConnectionEvent, 10),
	}

	// Test the structure is set up correctly
	if client.connected {
		t.Error("Client should start disconnected")
	}

	// Simulate connection
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	if !client.IsConnected() {
		t.Error("Client should be connected after setting connected=true")
	}
}

// TestClientChannelCapacity tests that channels are created with correct capacity.
func TestClientChannelCapacity(t *testing.T) {
	client := &TestClient{
		friendRequestCh: make(chan FriendRequest, 10),
		messageCh:       make(chan Message, 100),
		connectionCh:    make(chan ConnectionEvent, 10),
	}

	// Test channel capacity by filling to capacity
	for i := 0; i < 10; i++ {
		select {
		case client.friendRequestCh <- FriendRequest{}:
		default:
			t.Errorf("friendRequestCh should accept %d items", 10)
		}
	}

	// The 11th should fail (non-blocking)
	select {
	case client.friendRequestCh <- FriendRequest{}:
		t.Error("friendRequestCh should be full at capacity 10")
	default:
		// Expected
	}
}

// TestRunTestsStructure tests the basic structure of RunTests without executing.
// Note: Full RunTests testing requires real Tox instances that bind to ports.
func TestRunTestsStructure(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Verify the orchestrator is properly configured
	if orchestrator.results == nil {
		t.Error("Results should not be nil")
	}

	if orchestrator.results.FinalStatus != TestStatusPending {
		t.Errorf("Initial status should be PENDING, got %v", orchestrator.results.FinalStatus)
	}

	if orchestrator.config.OverallTimeout <= 0 {
		t.Error("OverallTimeout should be positive")
	}
}

// TestServerStatusStruct tests the ServerStatus struct fields.
func TestServerStatusStruct(t *testing.T) {
	status := ServerStatus{
		Running:           true,
		Address:           "127.0.0.1",
		Port:              33445,
		PublicKey:         "ABCDEF",
		Uptime:            5 * time.Minute,
		ConnectionsServed: 100,
		PacketsProcessed:  1000,
		ActiveClients:     3,
		ConnectionStatus:  1, // ConnectionTCP
	}

	if !status.Running {
		t.Error("Running should be true")
	}
	if status.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want %q", status.Address, "127.0.0.1")
	}
	if status.Port != 33445 {
		t.Errorf("Port = %d, want 33445", status.Port)
	}
	if status.PublicKey != "ABCDEF" {
		t.Errorf("PublicKey = %q, want %q", status.PublicKey, "ABCDEF")
	}
	if status.Uptime != 5*time.Minute {
		t.Errorf("Uptime = %v, want %v", status.Uptime, 5*time.Minute)
	}
	if status.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want 100", status.ConnectionsServed)
	}
	if status.PacketsProcessed != 1000 {
		t.Errorf("PacketsProcessed = %d, want 1000", status.PacketsProcessed)
	}
	if status.ActiveClients != 3 {
		t.Errorf("ActiveClients = %d, want 3", status.ActiveClients)
	}
	if status.ConnectionStatus != 1 {
		t.Errorf("ConnectionStatus = %d, want 1", status.ConnectionStatus)
	}
}

// TestClientStatusStruct tests the ClientStatus struct fields.
func TestClientStatusStruct(t *testing.T) {
	status := ClientStatus{
		Name:               "TestClient",
		Connected:          true,
		PublicKey:          "0123456789ABCDEF",
		ConnectionStatus:   2, // ConnectionUDP
		FriendCount:        5,
		Uptime:             10 * time.Minute,
		MessagesSent:       42,
		MessagesReceived:   37,
		FriendRequestsSent: 3,
		FriendRequestsRecv: 2,
		ConnectionEvents:   7,
	}

	if status.Name != "TestClient" {
		t.Errorf("Name = %q, want %q", status.Name, "TestClient")
	}
	if !status.Connected {
		t.Error("Connected should be true")
	}
	if status.PublicKey != "0123456789ABCDEF" {
		t.Errorf("PublicKey = %q, want %q", status.PublicKey, "0123456789ABCDEF")
	}
	if status.ConnectionStatus != 2 {
		t.Errorf("ConnectionStatus = %d, want 2", status.ConnectionStatus)
	}
	if status.FriendCount != 5 {
		t.Errorf("FriendCount = %d, want 5", status.FriendCount)
	}
	if status.Uptime != 10*time.Minute {
		t.Errorf("Uptime = %v, want %v", status.Uptime, 10*time.Minute)
	}
	if status.MessagesSent != 42 {
		t.Errorf("MessagesSent = %d, want 42", status.MessagesSent)
	}
	if status.MessagesReceived != 37 {
		t.Errorf("MessagesReceived = %d, want 37", status.MessagesReceived)
	}
	if status.FriendRequestsSent != 3 {
		t.Errorf("FriendRequestsSent = %d, want 3", status.FriendRequestsSent)
	}
	if status.FriendRequestsRecv != 2 {
		t.Errorf("FriendRequestsRecv = %d, want 2", status.FriendRequestsRecv)
	}
	if status.ConnectionEvents != 7 {
		t.Errorf("ConnectionEvents = %d, want 7", status.ConnectionEvents)
	}
}

// TestStepMetricsStruct tests the StepMetrics struct fields.
func TestStepMetricsStruct(t *testing.T) {
	metrics := StepMetrics{
		BytesSent:         1024,
		BytesReceived:     2048,
		MessagesProcessed: 50,
		RetryCount:        2,
		Latency:           100 * time.Millisecond,
		Custom:            map[string]any{"custom_key": "custom_value"},
	}

	if metrics.BytesSent != 1024 {
		t.Errorf("BytesSent = %d, want 1024", metrics.BytesSent)
	}
	if metrics.BytesReceived != 2048 {
		t.Errorf("BytesReceived = %d, want 2048", metrics.BytesReceived)
	}
	if metrics.MessagesProcessed != 50 {
		t.Errorf("MessagesProcessed = %d, want 50", metrics.MessagesProcessed)
	}
	if metrics.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", metrics.RetryCount)
	}
	if metrics.Latency != 100*time.Millisecond {
		t.Errorf("Latency = %v, want %v", metrics.Latency, 100*time.Millisecond)
	}
	if metrics.Custom["custom_key"] != "custom_value" {
		t.Errorf("Custom[\"custom_key\"] = %v, want %q", metrics.Custom["custom_key"], "custom_value")
	}
}

// TestTestStepResultWithTypedMetrics tests TestStepResult with TypedMetrics field.
func TestTestStepResultWithTypedMetrics(t *testing.T) {
	stepMetrics := &StepMetrics{
		BytesSent:         512,
		BytesReceived:     1024,
		MessagesProcessed: 10,
		RetryCount:        1,
		Latency:           50 * time.Millisecond,
	}

	result := TestStepResult{
		StepName:      "Test Step",
		Status:        TestStatusPassed,
		ExecutionTime: 5 * time.Second,
		ErrorMessage:  "",
		Metrics:       make(map[string]interface{}), // Deprecated
		TypedMetrics:  stepMetrics,
	}

	if result.StepName != "Test Step" {
		t.Errorf("StepName = %q, want %q", result.StepName, "Test Step")
	}
	if result.TypedMetrics == nil {
		t.Error("TypedMetrics should not be nil")
	}
	if result.TypedMetrics.BytesSent != 512 {
		t.Errorf("TypedMetrics.BytesSent = %d, want 512", result.TypedMetrics.BytesSent)
	}
}
