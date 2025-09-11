package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// TestOrchestrator manages the complete test execution workflow.
type TestOrchestrator struct {
	config    *TestConfig
	logger    *log.Logger
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
		BootstrapPort:        33445,
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

	// Set up logger
	logger := log.New(os.Stdout, "", log.LstdFlags)
	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.SetOutput(logFile)
	}

	return &TestOrchestrator{
		config: config,
		logger: logger,
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

	to.logger.Println("🧪 Tox Network Integration Test Suite")
	to.logger.Println("=====================================")
	to.logger.Printf("⏰ Test execution started at %s", to.startTime.Format(time.RFC3339))

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
			to.logger.Printf("⚠️  Cleanup warning: %v", err)
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

	to.logger.Printf("🎯 Executing: %s", stepName)

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
		to.logger.Printf("❌ %s failed: %v", stepName, err)
	} else {
		stepResult.Status = TestStatusPassed
		to.logger.Printf("✅ %s completed in %v", stepName, stepResult.ExecutionTime)
	}

	to.results.TestSteps = append(to.results.TestSteps, stepResult)
	return err
}

// logConfiguration prints the current test configuration.
func (to *TestOrchestrator) logConfiguration() {
	to.logger.Println("📋 Test Configuration:")
	to.logger.Printf("   Bootstrap: %s:%d", to.config.BootstrapAddress, to.config.BootstrapPort)
	to.logger.Printf("   Overall timeout: %v", to.config.OverallTimeout)
	to.logger.Printf("   Bootstrap timeout: %v", to.config.BootstrapTimeout)
	to.logger.Printf("   Connection timeout: %v", to.config.ConnectionTimeout)
	to.logger.Printf("   Friend request timeout: %v", to.config.FriendRequestTimeout)
	to.logger.Printf("   Message timeout: %v", to.config.MessageTimeout)
	to.logger.Printf("   Retry attempts: %d", to.config.RetryAttempts)
	to.logger.Printf("   Retry backoff: %v", to.config.RetryBackoff)
	to.logger.Printf("   Health checks: %v", to.config.EnableHealthChecks)
	to.logger.Printf("   Metrics collection: %v", to.config.CollectMetrics)
	to.logger.Println()
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
	to.logger.Println()
	to.logger.Println("📊 Test Execution Summary")
	to.logger.Println("========================")
}

// logOverallResults prints the overall test execution statistics.
func (to *TestOrchestrator) logOverallResults() {
	to.logger.Printf("🎯 Overall Status: %s", to.results.FinalStatus)
	to.logger.Printf("⏱️  Total Execution Time: %v", to.results.ExecutionTime)
	to.logger.Printf("📈 Tests: %d total, %d passed, %d failed, %d skipped",
		to.results.TotalTests, to.results.PassedTests, to.results.FailedTests, to.results.SkippedTests)
}

// logStepDetails prints detailed information about each test step.
func (to *TestOrchestrator) logStepDetails() {
	if len(to.results.TestSteps) == 0 {
		return
	}

	to.logger.Println("\n📋 Step Details:")
	for _, step := range to.results.TestSteps {
		statusIcon := to.getStatusIcon(step.Status)
		to.logger.Printf("   %s %s (%v)", statusIcon, step.StepName, step.ExecutionTime)

		if step.ErrorMessage != "" {
			to.logger.Printf("      Error: %s", step.ErrorMessage)
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

	to.logger.Println("\n❌ Error Details:")
	to.logger.Printf("   %s", to.results.ErrorDetails)
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
	to.logger.Println("\n🎉 All tests completed successfully!")
	to.logger.Println("✅ Tox protocol validation: PASSED")
	to.logger.Println("✅ Network connectivity: VERIFIED")
	to.logger.Println("✅ Friend requests: WORKING")
	to.logger.Println("✅ Message delivery: CONFIRMED")
}

// logFailureMessage prints failure messages for failed tests.
func (to *TestOrchestrator) logFailureMessage() {
	to.logger.Println("\n⚠️  Test execution completed with failures")
	to.logger.Println("   Review the error details above for troubleshooting")
}

// logReportFooter prints the test report footer with completion timestamp.
func (to *TestOrchestrator) logReportFooter() {
	to.logger.Printf("\n🏁 Test run completed at %s", time.Now().Format(time.RFC3339))
	to.logger.Println(strings.Repeat("=", 50))
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
	to.logger.SetOutput(output)
}

// SetVerbose enables or disables verbose logging.
func (to *TestOrchestrator) SetVerbose(verbose bool) {
	to.config.VerboseOutput = verbose
}
