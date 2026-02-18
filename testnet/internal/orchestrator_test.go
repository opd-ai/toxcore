package internal

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestTestStatusString tests the String method of TestStatus.
func TestTestStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   TestStatus
		expected string
	}{
		{"pending status", TestStatusPending, "PENDING"},
		{"running status", TestStatusRunning, "RUNNING"},
		{"passed status", TestStatusPassed, "PASSED"},
		{"failed status", TestStatusFailed, "FAILED"},
		{"skipped status", TestStatusSkipped, "SKIPPED"},
		{"timeout status", TestStatusTimeout, "TIMEOUT"},
		{"unknown status", TestStatus(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("TestStatus(%d).String() = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

// TestDefaultTestConfig tests the default configuration values.
func TestDefaultTestConfig(t *testing.T) {
	config := DefaultTestConfig()

	if config == nil {
		t.Fatal("DefaultTestConfig() returned nil")
	}

	// Verify default values
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"BootstrapPort", config.BootstrapPort, BootstrapDefaultPort},
		{"BootstrapAddress", config.BootstrapAddress, "127.0.0.1"},
		{"OverallTimeout", config.OverallTimeout, 5 * time.Minute},
		{"BootstrapTimeout", config.BootstrapTimeout, 10 * time.Second},
		{"ConnectionTimeout", config.ConnectionTimeout, 30 * time.Second},
		{"FriendRequestTimeout", config.FriendRequestTimeout, 15 * time.Second},
		{"MessageTimeout", config.MessageTimeout, 10 * time.Second},
		{"RetryAttempts", config.RetryAttempts, 3},
		{"RetryBackoff", config.RetryBackoff, time.Second},
		{"LogLevel", config.LogLevel, "INFO"},
		{"LogFile", config.LogFile, ""},
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

// TestNewTestOrchestratorWithNilConfig tests orchestrator creation with nil config.
func TestNewTestOrchestratorWithNilConfig(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator(nil) returned error: %v", err)
	}
	defer orchestrator.Cleanup()

	if orchestrator.config == nil {
		t.Error("Orchestrator config should not be nil")
	}

	// Should use default config
	if orchestrator.config.BootstrapPort != BootstrapDefaultPort {
		t.Errorf("Expected default BootstrapPort %d, got %d", BootstrapDefaultPort, orchestrator.config.BootstrapPort)
	}
}

// TestNewTestOrchestratorWithCustomConfig tests orchestrator creation with custom config.
func TestNewTestOrchestratorWithCustomConfig(t *testing.T) {
	config := &TestConfig{
		BootstrapPort:     44555,
		BootstrapAddress:  "192.168.1.1",
		OverallTimeout:    10 * time.Minute,
		ConnectionTimeout: 60 * time.Second,
		RetryAttempts:     5,
		RetryBackoff:      2 * time.Second,
	}

	orchestrator, err := NewTestOrchestrator(config)
	if err != nil {
		t.Fatalf("NewTestOrchestrator returned error: %v", err)
	}
	defer orchestrator.Cleanup()

	if orchestrator.config.BootstrapPort != 44555 {
		t.Errorf("Expected BootstrapPort 44555, got %d", orchestrator.config.BootstrapPort)
	}

	if orchestrator.config.BootstrapAddress != "192.168.1.1" {
		t.Errorf("Expected BootstrapAddress '192.168.1.1', got %s", orchestrator.config.BootstrapAddress)
	}

	if orchestrator.config.RetryAttempts != 5 {
		t.Errorf("Expected RetryAttempts 5, got %d", orchestrator.config.RetryAttempts)
	}
}

// TestNewTestOrchestratorWithLogFile tests orchestrator creation with log file.
func TestNewTestOrchestratorWithLogFile(t *testing.T) {
	// Create temp directory for log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := DefaultTestConfig()
	config.LogFile = logFile

	orchestrator, err := NewTestOrchestrator(config)
	if err != nil {
		t.Fatalf("NewTestOrchestrator returned error: %v", err)
	}
	defer orchestrator.Cleanup()

	// Verify log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}

	if orchestrator.logFile == nil {
		t.Error("Orchestrator logFile should not be nil when LogFile is specified")
	}
}

// TestNewTestOrchestratorWithInvalidLogFile tests orchestrator creation with invalid log file path.
func TestNewTestOrchestratorWithInvalidLogFile(t *testing.T) {
	config := DefaultTestConfig()
	config.LogFile = "/nonexistent/path/that/should/fail/test.log"

	_, err := NewTestOrchestrator(config)
	if err == nil {
		t.Error("NewTestOrchestrator should return error for invalid log file path")
	}
}

// TestValidateConfiguration tests the ValidateConfiguration method.
func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      *TestConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      DefaultTestConfig(),
			expectError: false,
		},
		{
			name: "zero bootstrap port",
			config: &TestConfig{
				BootstrapPort:     0,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "bootstrap port cannot be zero",
		},
		{
			name: "empty bootstrap address",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "bootstrap address cannot be empty",
		},
		{
			name: "zero overall timeout",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    0,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "overall timeout must be positive",
		},
		{
			name: "negative overall timeout",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    -time.Second,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "overall timeout must be positive",
		},
		{
			name: "zero connection timeout",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: 0,
				RetryAttempts:     1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "connection timeout must be positive",
		},
		{
			name: "negative retry attempts",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     -1,
				RetryBackoff:      time.Second,
			},
			expectError: true,
			errorMsg:    "retry attempts cannot be negative",
		},
		{
			name: "zero retry attempts is valid",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     0,
				RetryBackoff:      time.Second,
			},
			expectError: false,
		},
		{
			name: "zero retry backoff",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      0,
			},
			expectError: true,
			errorMsg:    "retry backoff must be positive",
		},
		{
			name: "negative retry backoff",
			config: &TestConfig{
				BootstrapPort:     BootstrapDefaultPort,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     1,
				RetryBackoff:      -time.Second,
			},
			expectError: true,
			errorMsg:    "retry backoff must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator, err := NewTestOrchestrator(tt.config)
			if err != nil {
				t.Fatalf("NewTestOrchestrator failed: %v", err)
			}
			defer orchestrator.Cleanup()

			err = orchestrator.ValidateConfiguration()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestOrchestratorCleanup tests the Cleanup method.
func TestOrchestratorCleanup(t *testing.T) {
	t.Run("cleanup without log file", func(t *testing.T) {
		orchestrator, err := NewTestOrchestrator(nil)
		if err != nil {
			t.Fatalf("NewTestOrchestrator failed: %v", err)
		}

		err = orchestrator.Cleanup()
		if err != nil {
			t.Errorf("Cleanup() returned error: %v", err)
		}
	})

	t.Run("cleanup with log file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		config := DefaultTestConfig()
		config.LogFile = logFile

		orchestrator, err := NewTestOrchestrator(config)
		if err != nil {
			t.Fatalf("NewTestOrchestrator failed: %v", err)
		}

		// Write something to ensure file is actually in use
		orchestrator.logger.Info("Test log entry")

		err = orchestrator.Cleanup()
		if err != nil {
			t.Errorf("Cleanup() returned error: %v", err)
		}

		if orchestrator.logFile != nil {
			t.Error("logFile should be nil after Cleanup()")
		}
	})

	t.Run("multiple cleanup calls are safe", func(t *testing.T) {
		orchestrator, err := NewTestOrchestrator(nil)
		if err != nil {
			t.Fatalf("NewTestOrchestrator failed: %v", err)
		}

		err = orchestrator.Cleanup()
		if err != nil {
			t.Errorf("First Cleanup() returned error: %v", err)
		}

		err = orchestrator.Cleanup()
		if err != nil {
			t.Errorf("Second Cleanup() returned error: %v", err)
		}
	})
}

// TestOrchestratorSetLogOutput tests the SetLogOutput method.
func TestOrchestratorSetLogOutput(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Create a buffer to capture log output
	var buf bytes.Buffer
	tempFile, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	orchestrator.SetLogOutput(tempFile)

	// Write a test message
	orchestrator.logger.Info("Test message")

	// Read back the content
	tempFile.Seek(0, io.SeekStart)
	content, _ := io.ReadAll(tempFile)

	if !bytes.Contains(content, []byte("Test message")) {
		t.Errorf("Expected log output to contain 'Test message', got %q", buf.String())
	}
}

// TestOrchestratorSetVerbose tests the SetVerbose method.
func TestOrchestratorSetVerbose(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	// Default should be true from DefaultTestConfig
	if !orchestrator.config.VerboseOutput {
		t.Error("Default VerboseOutput should be true")
	}

	orchestrator.SetVerbose(false)
	if orchestrator.config.VerboseOutput {
		t.Error("VerboseOutput should be false after SetVerbose(false)")
	}

	orchestrator.SetVerbose(true)
	if !orchestrator.config.VerboseOutput {
		t.Error("VerboseOutput should be true after SetVerbose(true)")
	}
}

// TestOrchestratorGetResults tests the GetResults method.
func TestOrchestratorGetResults(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	results := orchestrator.GetResults()
	if results == nil {
		t.Fatal("GetResults() returned nil")
	}

	if results.FinalStatus != TestStatusPending {
		t.Errorf("Initial FinalStatus should be PENDING, got %s", results.FinalStatus)
	}

	if results.TestSteps == nil {
		t.Error("TestSteps should not be nil")
	}
}

// TestOrchestratorGetStatusIcon tests the getStatusIcon method.
func TestOrchestratorGetStatusIcon(t *testing.T) {
	orchestrator, err := NewTestOrchestrator(nil)
	if err != nil {
		t.Fatalf("NewTestOrchestrator failed: %v", err)
	}
	defer orchestrator.Cleanup()

	tests := []struct {
		status   TestStatus
		expected string
	}{
		{TestStatusFailed, "❌"},
		{TestStatusSkipped, "⏭️"},
		{TestStatusPassed, "✅"},
		{TestStatusPending, "✅"},
		{TestStatusRunning, "✅"},
		{TestStatusTimeout, "✅"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			icon := orchestrator.getStatusIcon(tt.status)
			if icon != tt.expected {
				t.Errorf("getStatusIcon(%s) = %q, want %q", tt.status, icon, tt.expected)
			}
		})
	}
}

// TestTestStepResult tests the TestStepResult struct.
func TestTestStepResult(t *testing.T) {
	result := TestStepResult{
		StepName:      "Test Step",
		Status:        TestStatusPassed,
		ExecutionTime: 5 * time.Second,
		ErrorMessage:  "",
		Metrics:       map[string]interface{}{"key": "value"},
	}

	if result.StepName != "Test Step" {
		t.Errorf("StepName = %q, want %q", result.StepName, "Test Step")
	}

	if result.Status != TestStatusPassed {
		t.Errorf("Status = %s, want %s", result.Status, TestStatusPassed)
	}

	if result.ExecutionTime != 5*time.Second {
		t.Errorf("ExecutionTime = %v, want %v", result.ExecutionTime, 5*time.Second)
	}
}

// TestTestResults tests the TestResults struct initialization.
func TestTestResults(t *testing.T) {
	results := &TestResults{
		TotalTests:    10,
		PassedTests:   8,
		FailedTests:   1,
		SkippedTests:  1,
		ExecutionTime: 30 * time.Second,
		FinalStatus:   TestStatusPassed,
		ErrorDetails:  "",
	}

	if results.TotalTests != 10 {
		t.Errorf("TotalTests = %d, want %d", results.TotalTests, 10)
	}

	if results.PassedTests+results.FailedTests+results.SkippedTests != results.TotalTests {
		t.Error("Test counts don't add up to total")
	}
}

// TestDefaultProtocolConfig tests the default protocol configuration.
func TestDefaultProtocolConfig(t *testing.T) {
	config := DefaultProtocolConfig()

	if config == nil {
		t.Fatal("DefaultProtocolConfig() returned nil")
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"BootstrapTimeout", config.BootstrapTimeout, 10 * time.Second},
		{"ConnectionTimeout", config.ConnectionTimeout, 30 * time.Second},
		{"FriendRequestTimeout", config.FriendRequestTimeout, 15 * time.Second},
		{"MessageTimeout", config.MessageTimeout, 10 * time.Second},
		{"RetryAttempts", config.RetryAttempts, 3},
		{"RetryBackoff", config.RetryBackoff, time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	if config.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestNewProtocolTestSuiteWithNilConfig tests protocol test suite with nil config.
func TestNewProtocolTestSuiteWithNilConfig(t *testing.T) {
	suite := NewProtocolTestSuite(nil)

	if suite == nil {
		t.Fatal("NewProtocolTestSuite(nil) returned nil")
	}

	if suite.config == nil {
		t.Error("config should not be nil")
	}

	// Should use default config
	if suite.config.RetryAttempts != 3 {
		t.Errorf("Expected default RetryAttempts 3, got %d", suite.config.RetryAttempts)
	}
}

// TestNewProtocolTestSuiteWithCustomConfig tests protocol test suite with custom config.
func TestNewProtocolTestSuiteWithCustomConfig(t *testing.T) {
	customLogger := logrus.WithField("test", "custom")
	config := &ProtocolConfig{
		BootstrapTimeout:     20 * time.Second,
		ConnectionTimeout:    60 * time.Second,
		FriendRequestTimeout: 30 * time.Second,
		MessageTimeout:       20 * time.Second,
		RetryAttempts:        5,
		RetryBackoff:         2 * time.Second,
		Logger:               customLogger,
	}

	suite := NewProtocolTestSuite(config)

	if suite == nil {
		t.Fatal("NewProtocolTestSuite returned nil")
	}

	if suite.config.RetryAttempts != 5 {
		t.Errorf("Expected RetryAttempts 5, got %d", suite.config.RetryAttempts)
	}

	if suite.config.BootstrapTimeout != 20*time.Second {
		t.Errorf("Expected BootstrapTimeout 20s, got %v", suite.config.BootstrapTimeout)
	}

	if suite.logger != customLogger {
		t.Error("Logger was not set correctly")
	}
}
