package internal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// TestOrchestrator manages the complete test execution workflow.
type TestOrchestrator struct {
	config    *TestConfig
	logger    *logrus.Entry
	logFile   *os.File
	startTime time.Time
	results   *TestResults
}

// TestConfig holds configuration for the entire test suite.
type TestConfig struct {
	// Network configuration
	BootstrapPort    uint16
	BootstrapAddress string

	// Timeout configuration
	OverallTimeout       time.Duration
	BootstrapTimeout     time.Duration
	ConnectionTimeout    time.Duration
	FriendRequestTimeout time.Duration
	MessageTimeout       time.Duration

	// Retry configuration
	RetryAttempts int
	RetryBackoff  time.Duration

	// Logging configuration
	// LogLevel controls verbosity: "debug", "info", "warn", "error" (default: "info")
	LogLevel      string
	LogFile       string
	VerboseOutput bool

	// Test configuration
	EnableHealthChecks bool
	CollectMetrics     bool
}

// TestResults holds the outcomes of test execution.
type TestResults struct {
	TotalTests    int
	PassedTests   int
	FailedTests   int
	SkippedTests  int
	ExecutionTime time.Duration
	TestSteps     []TestStepResult
	FinalStatus   TestStatus
	ErrorDetails  string
}

// TestStepResult represents the result of an individual test step.
type TestStepResult struct {
	StepName      string
	Status        TestStatus
	ExecutionTime time.Duration
	ErrorMessage  string
	Metrics       map[string]interface{}
}

// TestStatus represents the status of a test or test step.
type TestStatus int

const (
	TestStatusPending TestStatus = iota
	TestStatusRunning
	TestStatusPassed
	TestStatusFailed
	TestStatusSkipped
	TestStatusTimeout
)

// String returns a string representation of the test status.
func (ts TestStatus) String() string {
	switch ts {
	case TestStatusPending:
		return "PENDING"
	case TestStatusRunning:
		return "RUNNING"
	case TestStatusPassed:
		return "PASSED"
	case TestStatusFailed:
		return "FAILED"
	case TestStatusSkipped:
		return "SKIPPED"
	case TestStatusTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

// DefaultTestConfig returns a default configuration for the test suite.
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		BootstrapPort:        BootstrapDefaultPort,
		BootstrapAddress:     "127.0.0.1",
		OverallTimeout:       5 * time.Minute,
		BootstrapTimeout:     10 * time.Second,
		ConnectionTimeout:    30 * time.Second,
		FriendRequestTimeout: 15 * time.Second,
		MessageTimeout:       10 * time.Second,
		RetryAttempts:        3,
		RetryBackoff:         time.Second,
		LogLevel:             "INFO",
		LogFile:              "",
		VerboseOutput:        true,
		EnableHealthChecks:   true,
		CollectMetrics:       true,
	}
}

// NewTestOrchestrator creates a new test orchestrator.
func NewTestOrchestrator(config *TestConfig) (*TestOrchestrator, error) {
	if config == nil {
		config = DefaultTestConfig()
	}

	// Set up logger with structured fields
	logger := logrus.WithField("component", "orchestrator")

	// Configure file output if specified
	var logFile *os.File
	if config.LogFile != "" {
		var err error
		logFile, err = os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logrus.SetOutput(logFile)
	}

	return &TestOrchestrator{
		config:  config,
		logger:  logger,
		logFile: logFile,
		results: &TestResults{
			TestSteps:   make([]TestStepResult, 0),
			FinalStatus: TestStatusPending,
		},
	}, nil
}

// RunTests executes the complete test suite.
func (to *TestOrchestrator) RunTests(ctx context.Context) (*TestResults, error) {
	to.startTime = time.Now()
	to.results.FinalStatus = TestStatusRunning

	to.logger.Info("🧪 Tox Network Integration Test Suite")
	to.logger.Info("=====================================")
	to.logger.WithField("start_time", to.startTime.Format(time.RFC3339)).Info("⏰ Test execution started")

	if to.config.VerboseOutput {
		to.logConfiguration()
	}

	// Create context with overall timeout
	testCtx, cancel := context.WithTimeout(ctx, to.config.OverallTimeout)
	defer cancel()

	// Execute test workflow with proper error handling
	err := to.executeTestWorkflow(testCtx)

	// Calculate final execution time
	to.results.ExecutionTime = time.Since(to.startTime)

	// Determine final status
	if err != nil {
		to.results.FinalStatus = TestStatusFailed
		to.results.ErrorDetails = err.Error()
		to.results.FailedTests = 1
	} else {
		to.results.FinalStatus = TestStatusPassed
		to.results.PassedTests = 1
	}
	to.results.TotalTests = 1

	// Generate final report
	to.generateFinalReport()

	return to.results, err
}

// executeTestWorkflow runs the core test workflow.
func (to *TestOrchestrator) executeTestWorkflow(ctx context.Context) error {
	// Create protocol test suite
	protocolConfig := &ProtocolConfig{
		BootstrapTimeout:     to.config.BootstrapTimeout,
		ConnectionTimeout:    to.config.ConnectionTimeout,
		FriendRequestTimeout: to.config.FriendRequestTimeout,
		MessageTimeout:       to.config.MessageTimeout,
		RetryAttempts:        to.config.RetryAttempts,
		RetryBackoff:         to.config.RetryBackoff,
		Logger:               to.logger,
	}

	protocolSuite := NewProtocolTestSuite(protocolConfig)
	defer func() {
		if err := protocolSuite.Cleanup(); err != nil {
			to.logger.WithError(err).Warn("⚠️  Cleanup warning")
		}
	}()

	// Execute the test with step tracking
	return to.executeWithStepTracking("Complete Protocol Test", func() error {
		return protocolSuite.ExecuteTest(ctx)
	})
}

// executeWithStepTracking executes a test step with result tracking.
func (to *TestOrchestrator) executeWithStepTracking(stepName string, operation func() error) error {
	stepStart := time.Now()

	to.logger.WithField("step", stepName).Info("🎯 Executing")

	stepResult := TestStepResult{
		StepName: stepName,
		Status:   TestStatusRunning,
		Metrics:  make(map[string]interface{}),
	}

	err := operation()

	stepResult.ExecutionTime = time.Since(stepStart)

	if err != nil {
		stepResult.Status = TestStatusFailed
		stepResult.ErrorMessage = err.Error()
		to.logger.WithFields(logrus.Fields{
			"step":  stepName,
			"error": err,
		}).Error("❌ Step failed")
	} else {
		stepResult.Status = TestStatusPassed
		to.logger.WithFields(logrus.Fields{
			"step":     stepName,
			"duration": stepResult.ExecutionTime,
		}).Info("✅ Step completed")
	}

	to.results.TestSteps = append(to.results.TestSteps, stepResult)
	return err
}

// logConfiguration prints the current test configuration.
func (to *TestOrchestrator) logConfiguration() {
	to.logger.Info("📋 Test Configuration:")
	to.logger.WithFields(logrus.Fields{
		"bootstrap_address":      to.config.BootstrapAddress,
		"bootstrap_port":         to.config.BootstrapPort,
		"overall_timeout":        to.config.OverallTimeout,
		"bootstrap_timeout":      to.config.BootstrapTimeout,
		"connection_timeout":     to.config.ConnectionTimeout,
		"friend_request_timeout": to.config.FriendRequestTimeout,
		"message_timeout":        to.config.MessageTimeout,
		"retry_attempts":         to.config.RetryAttempts,
		"retry_backoff":          to.config.RetryBackoff,
		"health_checks":          to.config.EnableHealthChecks,
		"metrics_collection":     to.config.CollectMetrics,
	}).Info("Configuration details")
}

// generateFinalReport creates and logs the final test report.
func (to *TestOrchestrator) generateFinalReport() {
	to.logReportHeader()
	to.logOverallResults()
	to.logStepDetails()
	to.logErrorDetails()
	to.logFinalStatus()
	to.logReportFooter()
}

// logReportHeader prints the test report header.
func (to *TestOrchestrator) logReportHeader() {
	to.logger.Info("")
	to.logger.Info("📊 Test Execution Summary")
	to.logger.Info("========================")
}

// logOverallResults prints the overall test execution statistics.
func (to *TestOrchestrator) logOverallResults() {
	to.logger.WithFields(logrus.Fields{
		"status":         to.results.FinalStatus,
		"execution_time": to.results.ExecutionTime,
		"total_tests":    to.results.TotalTests,
		"passed_tests":   to.results.PassedTests,
		"failed_tests":   to.results.FailedTests,
		"skipped_tests":  to.results.SkippedTests,
	}).Info("🎯 Overall Status")
}

// logStepDetails prints detailed information about each test step.
func (to *TestOrchestrator) logStepDetails() {
	if len(to.results.TestSteps) == 0 {
		return
	}

	to.logger.Info("📋 Step Details:")
	for _, step := range to.results.TestSteps {
		statusIcon := to.getStatusIcon(step.Status)
		to.logger.WithFields(logrus.Fields{
			"status":   statusIcon,
			"step":     step.StepName,
			"duration": step.ExecutionTime,
		}).Info("Step result")

		if step.ErrorMessage != "" {
			to.logger.WithField("error", step.ErrorMessage).Warn("Step error details")
		}
	}
}

// getStatusIcon returns the appropriate icon for a test status.
func (to *TestOrchestrator) getStatusIcon(status TestStatus) string {
	switch status {
	case TestStatusFailed:
		return "❌"
	case TestStatusSkipped:
		return "⏭️"
	default:
		return "✅"
	}
}

// logErrorDetails prints error details if any errors occurred.
func (to *TestOrchestrator) logErrorDetails() {
	if to.results.ErrorDetails == "" {
		return
	}

	to.logger.WithField("error", to.results.ErrorDetails).Error("❌ Error Details")
}

// logFinalStatus prints the final status message based on test results.
func (to *TestOrchestrator) logFinalStatus() {
	if to.results.FinalStatus == TestStatusPassed {
		to.logSuccessMessage()
	} else {
		to.logFailureMessage()
	}
}

// logSuccessMessage prints success messages for passed tests.
func (to *TestOrchestrator) logSuccessMessage() {
	to.logger.Info("🎉 All tests completed successfully!")
	to.logger.Info("✅ Tox protocol validation: PASSED")
	to.logger.Info("✅ Network connectivity: VERIFIED")
	to.logger.Info("✅ Friend requests: WORKING")
	to.logger.Info("✅ Message delivery: CONFIRMED")
}

// logFailureMessage prints failure messages for failed tests.
func (to *TestOrchestrator) logFailureMessage() {
	to.logger.Warn("⚠️  Test execution completed with failures")
	to.logger.Warn("   Review the error details above for troubleshooting")
}

// logReportFooter prints the test report footer with completion timestamp.
func (to *TestOrchestrator) logReportFooter() {
	to.logger.WithField("completed_at", time.Now().Format(time.RFC3339)).Info("🏁 Test run completed")
	to.logger.Info(strings.Repeat("=", 50))
}

// GetResults returns the current test results.
func (to *TestOrchestrator) GetResults() *TestResults {
	return to.results
}

// ValidateConfiguration validates the test configuration.
func (to *TestOrchestrator) ValidateConfiguration() error {
	if to.config.BootstrapPort == 0 {
		return fmt.Errorf("bootstrap port cannot be zero")
	}

	if to.config.BootstrapAddress == "" {
		return fmt.Errorf("bootstrap address cannot be empty")
	}

	if to.config.OverallTimeout <= 0 {
		return fmt.Errorf("overall timeout must be positive")
	}

	if to.config.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}

	if to.config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if to.config.RetryBackoff <= 0 {
		return fmt.Errorf("retry backoff must be positive")
	}

	return nil
}

// SetLogOutput configures the logger output destination.
func (to *TestOrchestrator) SetLogOutput(output *os.File) {
	logrus.SetOutput(output)
}

// SetVerbose enables or disables verbose logging.
func (to *TestOrchestrator) SetVerbose(verbose bool) {
	to.config.VerboseOutput = verbose
}

// Cleanup releases resources held by the orchestrator.
// This should be called when the orchestrator is no longer needed.
func (to *TestOrchestrator) Cleanup() error {
	if to.logFile != nil {
		if err := to.logFile.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		to.logFile = nil
	}
	return nil
}
