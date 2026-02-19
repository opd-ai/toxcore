package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/testnet/internal"
)

func TestValidateCLIConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *CLIConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with defaults",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				logLevel:             "INFO",
			},
			wantErr: false,
		},
		{
			name: "port zero",
			config: &CLIConfig{
				bootstrapPort:        0,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "invalid bootstrap port",
		},
		{
			name: "port over 65535",
			config: &CLIConfig{
				bootstrapPort:        70000,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "invalid bootstrap port",
		},
		{
			name: "empty address",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "bootstrap address cannot be empty",
		},
		{
			name: "zero overall timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       0,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "overall timeout must be positive",
		},
		{
			name: "negative overall timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       -1 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "overall timeout must be positive",
		},
		{
			name: "zero connection timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    0,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "connection timeout must be positive",
		},
		{
			name: "negative retry attempts",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        -1,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "retry attempts cannot be negative",
		},
		{
			name: "zero retry attempts is valid",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        0,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr: false,
		},
		{
			name: "zero retry backoff",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         0,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "retry backoff must be positive",
		},
		{
			name: "negative retry backoff",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         -1 * time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "retry backoff must be positive",
		},
		{
			name: "max valid port",
			config: &CLIConfig{
				bootstrapPort:        65535,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr: false,
		},
		{
			name: "min valid port",
			config: &CLIConfig{
				bootstrapPort:        1,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr: false,
		},
		{
			name: "zero bootstrap timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     0,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "bootstrap timeout must be positive",
		},
		{
			name: "zero friend request timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 0,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "friend request timeout must be positive",
		},
		{
			name: "zero message timeout",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       0,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
			},
			wantErr:     true,
			errContains: "message timeout must be positive",
		},
		{
			name: "invalid log level empty",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "",
			},
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name: "invalid log level unknown value",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "VERBOSE",
			},
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name: "valid log level DEBUG",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "DEBUG",
			},
			wantErr: false,
		},
		{
			name: "valid log level WARN",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "WARN",
			},
			wantErr: false,
		},
		{
			name: "valid log level ERROR",
			config: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "ERROR",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCLIConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCLIConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("validateCLIConfig() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestCreateTestConfig(t *testing.T) {
	tests := []struct {
		name      string
		cliConfig *CLIConfig
		want      *internal.TestConfig
	}{
		{
			name: "converts all fields correctly",
			cliConfig: &CLIConfig{
				bootstrapPort:        8080,
				bootstrapAddress:     "192.168.1.1",
				overallTimeout:       10 * time.Minute,
				bootstrapTimeout:     20 * time.Second,
				connectionTimeout:    60 * time.Second,
				friendRequestTimeout: 30 * time.Second,
				messageTimeout:       15 * time.Second,
				retryAttempts:        5,
				retryBackoff:         2 * time.Second,
				logLevel:             "DEBUG",
				logFile:              "/var/log/test.log",
				verbose:              true,
				enableHealthChecks:   false,
				collectMetrics:       true,
			},
			want: &internal.TestConfig{
				BootstrapPort:        8080,
				BootstrapAddress:     "192.168.1.1",
				OverallTimeout:       10 * time.Minute,
				BootstrapTimeout:     20 * time.Second,
				ConnectionTimeout:    60 * time.Second,
				FriendRequestTimeout: 30 * time.Second,
				MessageTimeout:       15 * time.Second,
				RetryAttempts:        5,
				RetryBackoff:         2 * time.Second,
				LogLevel:             "DEBUG",
				LogFile:              "/var/log/test.log",
				VerboseOutput:        true,
				EnableHealthChecks:   false,
				CollectMetrics:       true,
			},
		},
		{
			name: "handles default-like values",
			cliConfig: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              false,
				enableHealthChecks:   true,
				collectMetrics:       false,
			},
			want: &internal.TestConfig{
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
				VerboseOutput:        false,
				EnableHealthChecks:   true,
				CollectMetrics:       false,
			},
		},
		{
			name: "port truncation to uint16",
			cliConfig: &CLIConfig{
				bootstrapPort:        65535,
				bootstrapAddress:     "localhost",
				overallTimeout:       time.Minute,
				bootstrapTimeout:     time.Second,
				connectionTimeout:    time.Second,
				friendRequestTimeout: time.Second,
				messageTimeout:       time.Second,
				retryAttempts:        0,
				retryBackoff:         time.Millisecond,
				logLevel:             "ERROR",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
			},
			want: &internal.TestConfig{
				BootstrapPort:        65535,
				BootstrapAddress:     "localhost",
				OverallTimeout:       time.Minute,
				BootstrapTimeout:     time.Second,
				ConnectionTimeout:    time.Second,
				FriendRequestTimeout: time.Second,
				MessageTimeout:       time.Second,
				RetryAttempts:        0,
				RetryBackoff:         time.Millisecond,
				LogLevel:             "ERROR",
				LogFile:              "",
				VerboseOutput:        true,
				EnableHealthChecks:   true,
				CollectMetrics:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createTestConfig(tt.cliConfig)
			assertTestConfigEqual(t, got, tt.want)
		})
	}
}

func TestCLIConfigDefaults(t *testing.T) {
	// Test that CLIConfig struct initializes to zero values properly
	config := &CLIConfig{}

	if config.bootstrapPort != 0 {
		t.Errorf("expected bootstrapPort to be 0, got %d", config.bootstrapPort)
	}
	if config.bootstrapAddress != "" {
		t.Errorf("expected bootstrapAddress to be empty, got %q", config.bootstrapAddress)
	}
	if config.overallTimeout != 0 {
		t.Errorf("expected overallTimeout to be 0, got %v", config.overallTimeout)
	}
	if config.help != false {
		t.Errorf("expected help to be false, got %v", config.help)
	}
}

// Helper functions

// contains checks if substr is contained within s using stdlib strings.Contains.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func assertTestConfigEqual(t *testing.T, got, want *internal.TestConfig) {
	t.Helper()

	if got.BootstrapPort != want.BootstrapPort {
		t.Errorf("BootstrapPort = %d, want %d", got.BootstrapPort, want.BootstrapPort)
	}
	if got.BootstrapAddress != want.BootstrapAddress {
		t.Errorf("BootstrapAddress = %q, want %q", got.BootstrapAddress, want.BootstrapAddress)
	}
	if got.OverallTimeout != want.OverallTimeout {
		t.Errorf("OverallTimeout = %v, want %v", got.OverallTimeout, want.OverallTimeout)
	}
	if got.BootstrapTimeout != want.BootstrapTimeout {
		t.Errorf("BootstrapTimeout = %v, want %v", got.BootstrapTimeout, want.BootstrapTimeout)
	}
	if got.ConnectionTimeout != want.ConnectionTimeout {
		t.Errorf("ConnectionTimeout = %v, want %v", got.ConnectionTimeout, want.ConnectionTimeout)
	}
	if got.FriendRequestTimeout != want.FriendRequestTimeout {
		t.Errorf("FriendRequestTimeout = %v, want %v", got.FriendRequestTimeout, want.FriendRequestTimeout)
	}
	if got.MessageTimeout != want.MessageTimeout {
		t.Errorf("MessageTimeout = %v, want %v", got.MessageTimeout, want.MessageTimeout)
	}
	if got.RetryAttempts != want.RetryAttempts {
		t.Errorf("RetryAttempts = %d, want %d", got.RetryAttempts, want.RetryAttempts)
	}
	if got.RetryBackoff != want.RetryBackoff {
		t.Errorf("RetryBackoff = %v, want %v", got.RetryBackoff, want.RetryBackoff)
	}
	if got.LogLevel != want.LogLevel {
		t.Errorf("LogLevel = %q, want %q", got.LogLevel, want.LogLevel)
	}
	if got.LogFile != want.LogFile {
		t.Errorf("LogFile = %q, want %q", got.LogFile, want.LogFile)
	}
	if got.VerboseOutput != want.VerboseOutput {
		t.Errorf("VerboseOutput = %v, want %v", got.VerboseOutput, want.VerboseOutput)
	}
	if got.EnableHealthChecks != want.EnableHealthChecks {
		t.Errorf("EnableHealthChecks = %v, want %v", got.EnableHealthChecks, want.EnableHealthChecks)
	}
	if got.CollectMetrics != want.CollectMetrics {
		t.Errorf("CollectMetrics = %v, want %v", got.CollectMetrics, want.CollectMetrics)
	}
}

func TestPrintUsage(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify expected content is present
	expectedStrings := []string{
		"Tox Network Integration Test Suite",
		"Bootstrap server initialization",
		"Usage:",
		"Options:",
		"Examples:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("printUsage() output missing %q", expected)
		}
	}
}

func TestSetupSignalHandling(t *testing.T) {
	// Test that setupSignalHandling creates signal handler without panic
	// We can't fully test signal handling in unit tests, but we can
	// verify it doesn't panic when setting up
	cancel := func() {}

	// This should not panic
	setupSignalHandling(cancel)

	// Signal handling is asynchronous, we just verify setup completed
	// The actual signal handling would need integration tests
}

func TestOrchestratorCleanup(t *testing.T) {
	// Test that orchestrator cleanup works correctly
	config := internal.DefaultTestConfig()

	orchestrator, err := internal.NewTestOrchestrator(config)
	if err != nil {
		t.Fatalf("NewTestOrchestrator() error = %v", err)
	}

	// Cleanup should not error for default config (no log file)
	if err := orchestrator.Cleanup(); err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Double cleanup should also not error
	if err := orchestrator.Cleanup(); err != nil {
		t.Errorf("Double Cleanup() error = %v", err)
	}
}

func TestOrchestratorCleanupWithLogFile(t *testing.T) {
	// Create a temporary log file
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	config := internal.DefaultTestConfig()
	config.LogFile = tmpPath

	orchestrator, err := internal.NewTestOrchestrator(config)
	if err != nil {
		t.Fatalf("NewTestOrchestrator() error = %v", err)
	}

	// Cleanup should close the log file
	if err := orchestrator.Cleanup(); err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}
}

func TestOrchestratorValidateConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *internal.TestConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  internal.DefaultTestConfig(),
			wantErr: false,
		},
		{
			name: "invalid zero port",
			config: &internal.TestConfig{
				BootstrapPort:     0,
				BootstrapAddress:  "127.0.0.1",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     3,
				RetryBackoff:      time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid empty address",
			config: &internal.TestConfig{
				BootstrapPort:     33445,
				BootstrapAddress:  "",
				OverallTimeout:    time.Minute,
				ConnectionTimeout: time.Second,
				RetryAttempts:     3,
				RetryBackoff:      time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator, err := internal.NewTestOrchestrator(tt.config)
			if err != nil {
				t.Fatalf("NewTestOrchestrator() error = %v", err)
			}
			defer orchestrator.Cleanup()

			err = orchestrator.ValidateConfiguration()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseCLIFlags(t *testing.T) {
	// Save original os.Args and flag.CommandLine
	originalArgs := os.Args
	originalFlagCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagCommandLine
	}()

	tests := []struct {
		name     string
		args     []string
		expected *CLIConfig
	}{
		{
			name: "default values",
			args: []string{"cmd"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 false,
			},
		},
		{
			name: "custom port and address",
			args: []string{"cmd", "-port", "8080", "-address", "192.168.1.1"},
			expected: &CLIConfig{
				bootstrapPort:        8080,
				bootstrapAddress:     "192.168.1.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 false,
			},
		},
		{
			name: "custom timeouts",
			args: []string{"cmd", "-overall-timeout", "10m", "-connection-timeout", "1m"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       10 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    60 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 false,
			},
		},
		{
			name: "custom retry settings",
			args: []string{"cmd", "-retry-attempts", "5", "-retry-backoff", "2s"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        5,
				retryBackoff:         2 * time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 false,
			},
		},
		{
			name: "custom logging",
			args: []string{"cmd", "-log-level", "DEBUG", "-log-file", "/tmp/test.log", "-verbose=false"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "DEBUG",
				logFile:              "/tmp/test.log",
				verbose:              false,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 false,
			},
		},
		{
			name: "help flag",
			args: []string{"cmd", "-help"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   true,
				collectMetrics:       true,
				help:                 true,
			},
		},
		{
			name: "disable features",
			args: []string{"cmd", "-health-checks=false", "-metrics=false"},
			expected: &CLIConfig{
				bootstrapPort:        33445,
				bootstrapAddress:     "127.0.0.1",
				overallTimeout:       5 * time.Minute,
				bootstrapTimeout:     10 * time.Second,
				connectionTimeout:    30 * time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				logLevel:             "INFO",
				logFile:              "",
				verbose:              true,
				enableHealthChecks:   false,
				collectMetrics:       false,
				help:                 false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			os.Args = tt.args

			config := parseCLIFlags()

			if config.bootstrapPort != tt.expected.bootstrapPort {
				t.Errorf("bootstrapPort = %d, want %d", config.bootstrapPort, tt.expected.bootstrapPort)
			}
			if config.bootstrapAddress != tt.expected.bootstrapAddress {
				t.Errorf("bootstrapAddress = %q, want %q", config.bootstrapAddress, tt.expected.bootstrapAddress)
			}
			if config.overallTimeout != tt.expected.overallTimeout {
				t.Errorf("overallTimeout = %v, want %v", config.overallTimeout, tt.expected.overallTimeout)
			}
			if config.bootstrapTimeout != tt.expected.bootstrapTimeout {
				t.Errorf("bootstrapTimeout = %v, want %v", config.bootstrapTimeout, tt.expected.bootstrapTimeout)
			}
			if config.connectionTimeout != tt.expected.connectionTimeout {
				t.Errorf("connectionTimeout = %v, want %v", config.connectionTimeout, tt.expected.connectionTimeout)
			}
			if config.friendRequestTimeout != tt.expected.friendRequestTimeout {
				t.Errorf("friendRequestTimeout = %v, want %v", config.friendRequestTimeout, tt.expected.friendRequestTimeout)
			}
			if config.messageTimeout != tt.expected.messageTimeout {
				t.Errorf("messageTimeout = %v, want %v", config.messageTimeout, tt.expected.messageTimeout)
			}
			if config.retryAttempts != tt.expected.retryAttempts {
				t.Errorf("retryAttempts = %d, want %d", config.retryAttempts, tt.expected.retryAttempts)
			}
			if config.retryBackoff != tt.expected.retryBackoff {
				t.Errorf("retryBackoff = %v, want %v", config.retryBackoff, tt.expected.retryBackoff)
			}
			if config.logLevel != tt.expected.logLevel {
				t.Errorf("logLevel = %q, want %q", config.logLevel, tt.expected.logLevel)
			}
			if config.logFile != tt.expected.logFile {
				t.Errorf("logFile = %q, want %q", config.logFile, tt.expected.logFile)
			}
			if config.verbose != tt.expected.verbose {
				t.Errorf("verbose = %v, want %v", config.verbose, tt.expected.verbose)
			}
			if config.enableHealthChecks != tt.expected.enableHealthChecks {
				t.Errorf("enableHealthChecks = %v, want %v", config.enableHealthChecks, tt.expected.enableHealthChecks)
			}
			if config.collectMetrics != tt.expected.collectMetrics {
				t.Errorf("collectMetrics = %v, want %v", config.collectMetrics, tt.expected.collectMetrics)
			}
			if config.help != tt.expected.help {
				t.Errorf("help = %v, want %v", config.help, tt.expected.help)
			}
		})
	}
}

// runWithArgs is a test helper that executes run() with custom arguments.
// It returns the exit code and captures any stdout/stderr output.
func runWithArgs(args []string) (exitCode int, stdout, stderr string) {
	// Save original state
	originalArgs := os.Args
	originalFlagCommandLine := flag.CommandLine
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagCommandLine
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	// Set up args
	os.Args = args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)

	// Capture stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Run
	exitCode = run()

	// Read captured output
	wOut.Close()
	wErr.Close()

	var outBuf, errBuf bytes.Buffer
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)

	return exitCode, outBuf.String(), errBuf.String()
}

func TestRunWithHelpFlag(t *testing.T) {
	exitCode, stdout, _ := runWithArgs([]string{"cmd", "-help"})

	if exitCode != 0 {
		t.Errorf("run() with -help returned exit code %d, want 0", exitCode)
	}

	if !strings.Contains(stdout, "Tox Network Integration Test Suite") {
		t.Errorf("run() with -help did not print usage information")
	}
}

func TestRunWithInvalidConfig(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "invalid port zero",
			args:     []string{"cmd", "-port", "0"},
			wantCode: 1,
		},
		{
			name:     "invalid empty address",
			args:     []string{"cmd", "-address", ""},
			wantCode: 1,
		},
		{
			name:     "invalid log level",
			args:     []string{"cmd", "-log-level", "INVALID"},
			wantCode: 1,
		},
		{
			name:     "negative retry attempts",
			args:     []string{"cmd", "-retry-attempts", "-1"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, _, stderr := runWithArgs(tt.args)

			if exitCode != tt.wantCode {
				t.Errorf("run() exit code = %d, want %d", exitCode, tt.wantCode)
			}

			if !strings.Contains(stderr, "Use -help for usage information") {
				t.Errorf("run() stderr should contain help hint")
			}
		})
	}
}

func TestValidLogLevels(t *testing.T) {
	// Verify validLogLevels map contains expected entries
	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for _, level := range expectedLevels {
		if !validLogLevels[level] {
			t.Errorf("validLogLevels missing expected level %q", level)
		}
	}

	// Verify it doesn't contain invalid levels
	invalidLevels := []string{"TRACE", "FATAL", "VERBOSE", ""}
	for _, level := range invalidLevels {
		if validLogLevels[level] {
			t.Errorf("validLogLevels unexpectedly contains invalid level %q", level)
		}
	}
}
