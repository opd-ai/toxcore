package main

import (
	"bytes"
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
				connectionTimeout:    30 * time.Second,
				retryAttempts:        3,
				retryBackoff:         time.Second,
				friendRequestTimeout: 15 * time.Second,
				messageTimeout:       10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "port zero",
			config: &CLIConfig{
				bootstrapPort:     0,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "invalid bootstrap port",
		},
		{
			name: "port over 65535",
			config: &CLIConfig{
				bootstrapPort:     70000,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "invalid bootstrap port",
		},
		{
			name: "empty address",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "bootstrap address cannot be empty",
		},
		{
			name: "zero overall timeout",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    0,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "overall timeout must be positive",
		},
		{
			name: "negative overall timeout",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    -1 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "overall timeout must be positive",
		},
		{
			name: "zero connection timeout",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 0,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "connection timeout must be positive",
		},
		{
			name: "negative retry attempts",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     -1,
				retryBackoff:      time.Second,
			},
			wantErr:     true,
			errContains: "retry attempts cannot be negative",
		},
		{
			name: "zero retry attempts is valid",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     0,
				retryBackoff:      time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero retry backoff",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      0,
			},
			wantErr:     true,
			errContains: "retry backoff must be positive",
		},
		{
			name: "negative retry backoff",
			config: &CLIConfig{
				bootstrapPort:     33445,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      -1 * time.Second,
			},
			wantErr:     true,
			errContains: "retry backoff must be positive",
		},
		{
			name: "max valid port",
			config: &CLIConfig{
				bootstrapPort:     65535,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
			},
			wantErr: false,
		},
		{
			name: "min valid port",
			config: &CLIConfig{
				bootstrapPort:     1,
				bootstrapAddress:  "127.0.0.1",
				overallTimeout:    5 * time.Minute,
				connectionTimeout: 30 * time.Second,
				retryAttempts:     3,
				retryBackoff:      time.Second,
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
