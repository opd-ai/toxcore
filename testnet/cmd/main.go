// Package main provides the command-line interface for the Tox network integration test suite.
//
// This executable runs comprehensive tests to validate core Tox protocol operations
// through complete peer-to-peer communication workflows, including bootstrap server
// initialization, client management, friend connections, and message exchange.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/opd-ai/toxcore/testnet/internal"
	"github.com/sirupsen/logrus"
)

// CLIConfig holds command-line configuration options for the test suite.
// It contains network settings, timeout values, retry behavior, and logging options.
type CLIConfig struct {
	bootstrapPort        uint
	bootstrapAddress     string
	overallTimeout       time.Duration
	bootstrapTimeout     time.Duration
	connectionTimeout    time.Duration
	friendRequestTimeout time.Duration
	messageTimeout       time.Duration
	retryAttempts        int
	retryBackoff         time.Duration
	logLevel             string
	logFile              string
	verbose              bool
	enableHealthChecks   bool
	collectMetrics       bool
	help                 bool
}

// parseCLIFlags parses command-line flags and returns the configuration.
// The returned CLIConfig contains all parsed flag values with their defaults.
// Network flags: -port, -address
// Timeout flags: -overall-timeout, -bootstrap-timeout, -connection-timeout, -friend-request-timeout, -message-timeout
// Retry flags: -retry-attempts, -retry-backoff
// Logging flags: -log-level, -log-file, -verbose
// Feature flags: -health-checks, -metrics
// Help flag: -help
func parseCLIFlags() *CLIConfig {
	config := &CLIConfig{}

	// Network configuration
	flag.UintVar(&config.bootstrapPort, "port", 33445, "Bootstrap server port")
	flag.StringVar(&config.bootstrapAddress, "address", "127.0.0.1", "Bootstrap server address")

	// Timeout configuration
	flag.DurationVar(&config.overallTimeout, "overall-timeout", 5*time.Minute, "Overall test timeout")
	flag.DurationVar(&config.bootstrapTimeout, "bootstrap-timeout", 10*time.Second, "Bootstrap server startup timeout")
	flag.DurationVar(&config.connectionTimeout, "connection-timeout", 30*time.Second, "Client connection timeout")
	flag.DurationVar(&config.friendRequestTimeout, "friend-request-timeout", 15*time.Second, "Friend request timeout")
	flag.DurationVar(&config.messageTimeout, "message-timeout", 10*time.Second, "Message delivery timeout")

	// Retry configuration
	flag.IntVar(&config.retryAttempts, "retry-attempts", 3, "Number of retry attempts for operations")
	flag.DurationVar(&config.retryBackoff, "retry-backoff", time.Second, "Initial backoff duration for retries")

	// Logging configuration
	flag.StringVar(&config.logLevel, "log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
	flag.StringVar(&config.logFile, "log-file", "", "Log file path (default: stdout)")
	flag.BoolVar(&config.verbose, "verbose", true, "Enable verbose output")

	// Feature flags
	flag.BoolVar(&config.enableHealthChecks, "health-checks", true, "Enable health checks")
	flag.BoolVar(&config.collectMetrics, "metrics", true, "Enable metrics collection")

	// Help
	flag.BoolVar(&config.help, "help", false, "Show help message")

	flag.Parse()
	return config
}

// printUsage prints the usage information.
func printUsage() {
	fmt.Println("Tox Network Integration Test Suite")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("This tool validates core Tox protocol operations through a complete")
	fmt.Println("peer-to-peer communication workflow including:")
	fmt.Println("  • Bootstrap server initialization")
	fmt.Println("  • Client connection and peer discovery")
	fmt.Println("  • Friend request exchange")
	fmt.Println("  • Bidirectional message delivery")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  # Run with default settings\n")
	fmt.Printf("  %s\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Run with custom port and timeouts\n")
	fmt.Printf("  %s -port 8080 -connection-timeout 60s -verbose\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Run with log file and reduced verbosity\n")
	fmt.Printf("  %s -log-file test.log -verbose=false\n", os.Args[0])
}

// validLogLevels contains the allowed log level values.
var validLogLevels = map[string]bool{
	"DEBUG": true,
	"INFO":  true,
	"WARN":  true,
	"ERROR": true,
}

// validateCLIConfig validates the CLI configuration.
func validateCLIConfig(config *CLIConfig) error {
	if err := validatePortAndAddress(config); err != nil {
		return err
	}

	if err := validateTimeouts(config); err != nil {
		return err
	}

	if err := validateRetrySettings(config); err != nil {
		return err
	}

	if !validLogLevels[config.logLevel] {
		return fmt.Errorf("invalid log level %q: must be one of DEBUG, INFO, WARN, ERROR", config.logLevel)
	}

	return nil
}

// validatePortAndAddress validates bootstrap port and address configuration.
func validatePortAndAddress(config *CLIConfig) error {
	if config.bootstrapPort == 0 || config.bootstrapPort > 65535 {
		return fmt.Errorf("invalid bootstrap port: must be between 1 and 65535")
	}

	if config.bootstrapAddress == "" {
		return fmt.Errorf("bootstrap address cannot be empty")
	}

	return nil
}

// validateTimeouts validates all timeout-related configuration values.
func validateTimeouts(config *CLIConfig) error {
	if config.overallTimeout <= 0 {
		return fmt.Errorf("overall timeout must be positive")
	}

	if config.bootstrapTimeout <= 0 {
		return fmt.Errorf("bootstrap timeout must be positive")
	}

	if config.connectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}

	if config.friendRequestTimeout <= 0 {
		return fmt.Errorf("friend request timeout must be positive")
	}

	if config.messageTimeout <= 0 {
		return fmt.Errorf("message timeout must be positive")
	}

	return nil
}

// validateRetrySettings validates retry attempts and backoff configuration.
func validateRetrySettings(config *CLIConfig) error {
	if config.retryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if config.retryBackoff <= 0 {
		return fmt.Errorf("retry backoff must be positive")
	}

	return nil
}

	return nil
}

// createTestConfig converts CLI configuration to internal test configuration.
func createTestConfig(cliConfig *CLIConfig) *internal.TestConfig {
	return &internal.TestConfig{
		BootstrapPort:        uint16(cliConfig.bootstrapPort),
		BootstrapAddress:     cliConfig.bootstrapAddress,
		OverallTimeout:       cliConfig.overallTimeout,
		BootstrapTimeout:     cliConfig.bootstrapTimeout,
		ConnectionTimeout:    cliConfig.connectionTimeout,
		FriendRequestTimeout: cliConfig.friendRequestTimeout,
		MessageTimeout:       cliConfig.messageTimeout,
		RetryAttempts:        cliConfig.retryAttempts,
		RetryBackoff:         cliConfig.retryBackoff,
		LogLevel:             cliConfig.logLevel,
		LogFile:              cliConfig.logFile,
		VerboseOutput:        cliConfig.verbose,
		EnableHealthChecks:   cliConfig.enableHealthChecks,
		CollectMetrics:       cliConfig.collectMetrics,
	}
}

// setupSignalHandling sets up graceful shutdown on interrupt signals.
func setupSignalHandling(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		sig := <-sigChan
		logrus.WithFields(logrus.Fields{
			"signal":  sig.String(),
			"context": "signal_handling",
		}).Info("Received interrupt signal, initiating graceful shutdown")
		cancel()
	}()
}

// main is the entry point for the test suite.
func main() {
	os.Exit(run())
}

// run executes the main application logic and returns an exit code.
// This allows deferred cleanup to run properly.
func run() int {
	// Parse command-line flags
	cliConfig := parseCLIFlags()

	// Show help if requested
	if cliConfig.help {
		printUsage()
		return 0
	}

	// Validate configuration
	if err := validateCLIConfig(cliConfig); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":   err.Error(),
			"context": "configuration_validation",
		}).Error("Configuration error")
		fmt.Fprintln(os.Stderr, "Use -help for usage information.")
		return 1
	}

	// Create test configuration
	testConfig := createTestConfig(cliConfig)

	// Create test orchestrator
	orchestrator, err := internal.NewTestOrchestrator(testConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":           fmt.Errorf("orchestrator creation failed: %w", err).Error(),
			"bootstrap_port":  cliConfig.bootstrapPort,
			"bootstrap_addr":  cliConfig.bootstrapAddress,
			"overall_timeout": cliConfig.overallTimeout.String(),
			"context":         "orchestrator_creation",
		}).Error("Failed to create test orchestrator")
		return 1
	}
	defer func() {
		if cleanupErr := orchestrator.Cleanup(); cleanupErr != nil {
			logrus.WithFields(logrus.Fields{
				"error":   fmt.Errorf("cleanup failed: %w", cleanupErr).Error(),
				"context": "orchestrator_cleanup",
			}).Warn("Cleanup warning")
		}
	}()

	// Validate orchestrator configuration
	if err := orchestrator.ValidateConfiguration(); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":   fmt.Errorf("configuration validation failed: %w", err).Error(),
			"context": "configuration_validation",
		}).Error("Invalid orchestrator configuration")
		return 1
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling(cancel)

	// Run the tests
	fmt.Println("🚀 Starting Tox Network Integration Test Suite...")
	fmt.Println()

	results, err := orchestrator.RunTests(ctx)

	// Determine exit code based on results
	exitCode := 0
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":   fmt.Errorf("test execution failed: %w", err).Error(),
			"context": "test_execution",
		}).Error("Test execution failed")
		exitCode = 1
	} else if results.FinalStatus != internal.TestStatusPassed {
		logrus.WithFields(logrus.Fields{
			"final_status": results.FinalStatus.String(),
			"total_tests":  results.TotalTests,
			"passed_tests": results.PassedTests,
			"failed_tests": results.FailedTests,
			"context":      "test_completion",
		}).Error("Test suite completed with failures")
		exitCode = 1
	} else {
		fmt.Println("\n🎉 Test suite completed successfully!")
	}

	// Print summary information
	if results != nil {
		fmt.Printf("\n📊 Summary: %d tests, %d passed, %d failed (execution time: %v)\n",
			results.TotalTests, results.PassedTests, results.FailedTests, results.ExecutionTime)
	}

	return exitCode
}
