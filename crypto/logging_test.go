package crypto

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestNewLogger tests the NewLogger function
func TestNewLogger(t *testing.T) {
	tests := []struct {
		name         string
		function     string
		expectedFunc string
		expectedPkg  string
	}{
		{
			name:         "basic function",
			function:     "TestFunction",
			expectedFunc: "TestFunction",
			expectedPkg:  "crypto",
		},
		{
			name:         "empty function",
			function:     "",
			expectedFunc: "",
			expectedPkg:  "crypto",
		},
		{
			name:         "complex function name",
			function:     "ComplexFunctionNameWithMultipleWords",
			expectedFunc: "ComplexFunctionNameWithMultipleWords",
			expectedPkg:  "crypto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.function)

			if logger.function != tt.expectedFunc {
				t.Errorf("NewLogger() function = %v, want %v", logger.function, tt.expectedFunc)
			}

			if logger.pkg != tt.expectedPkg {
				t.Errorf("NewLogger() pkg = %v, want %v", logger.pkg, tt.expectedPkg)
			}

			if logger.fields["function"] != tt.expectedFunc {
				t.Errorf("NewLogger() fields[function] = %v, want %v", logger.fields["function"], tt.expectedFunc)
			}

			if logger.fields["package"] != tt.expectedPkg {
				t.Errorf("NewLogger() fields[package] = %v, want %v", logger.fields["package"], tt.expectedPkg)
			}
		})
	}
}

// TestLoggerHelper_WithCaller tests the WithCaller method
func TestLoggerHelper_WithCaller(t *testing.T) {
	logger := NewLogger("TestFunction")
	loggerWithCaller := logger.WithCaller()

	// Verify that caller information was added
	if _, exists := loggerWithCaller.fields["caller"]; !exists {
		t.Error("WithCaller() should add caller field")
	}

	if _, exists := loggerWithCaller.fields["caller_func"]; !exists {
		t.Error("WithCaller() should add caller_func field")
	}

	// Verify caller contains file and line information
	caller, ok := loggerWithCaller.fields["caller"].(string)
	if !ok {
		t.Error("WithCaller() caller field should be string")
	}

	if !strings.Contains(caller, ":") {
		t.Error("WithCaller() caller should contain file:line format")
	}

	// Verify caller_func contains function name
	callerFunc, ok := loggerWithCaller.fields["caller_func"].(string)
	if !ok {
		t.Error("WithCaller() caller_func field should be string")
	}

	if len(callerFunc) == 0 {
		t.Error("WithCaller() caller_func should not be empty")
	}

	// Test method chaining returns same instance
	if loggerWithCaller != logger {
		t.Error("WithCaller() should return same logger instance for chaining")
	}
}

// TestLoggerHelper_WithField tests the WithField method
func TestLoggerHelper_WithField(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "string value",
			key:   "test_string",
			value: "test_value",
		},
		{
			name:  "integer value",
			key:   "test_int",
			value: 42,
		},
		{
			name:  "boolean value",
			key:   "test_bool",
			value: true,
		},
		{
			name:  "nil value",
			key:   "test_nil",
			value: nil,
		},
		{
			name:  "empty key",
			key:   "",
			value: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger("TestFunction")
			loggerWithField := logger.WithField(tt.key, tt.value)

			// Verify field was added
			if val, exists := loggerWithField.fields[tt.key]; !exists {
				t.Errorf("WithField() should add field %s", tt.key)
			} else if val != tt.value {
				t.Errorf("WithField() field value = %v, want %v", val, tt.value)
			}

			// Test method chaining returns same instance
			if loggerWithField != logger {
				t.Error("WithField() should return same logger instance for chaining")
			}
		})
	}
}

// TestLoggerHelper_WithFields tests the WithFields method
func TestLoggerHelper_WithFields(t *testing.T) {
	tests := []struct {
		name   string
		fields logrus.Fields
	}{
		{
			name: "single field",
			fields: logrus.Fields{
				"key1": "value1",
			},
		},
		{
			name: "multiple fields",
			fields: logrus.Fields{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
		{
			name:   "empty fields",
			fields: logrus.Fields{},
		},
		{
			name:   "nil fields",
			fields: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger("TestFunction")
			loggerWithFields := logger.WithFields(tt.fields)

			// Verify all fields were added
			for key, expectedValue := range tt.fields {
				if val, exists := loggerWithFields.fields[key]; !exists {
					t.Errorf("WithFields() should add field %s", key)
				} else if val != expectedValue {
					t.Errorf("WithFields() field %s value = %v, want %v", key, val, expectedValue)
				}
			}

			// Test method chaining returns same instance
			if loggerWithFields != logger {
				t.Error("WithFields() should return same logger instance for chaining")
			}
		})
	}
}

// TestLoggerHelper_WithError tests the WithError method
func TestLoggerHelper_WithError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		errorType string
		operation string
	}{
		{
			name:      "basic error",
			err:       errors.New("test error"),
			errorType: "test_error",
			operation: "test_operation",
		},
		{
			name:      "formatted error",
			err:       fmt.Errorf("formatted error: %w", errors.New("underlying error")),
			errorType: "formatted_error",
			operation: "format_operation",
		},
		{
			name:      "empty strings",
			err:       errors.New("error"),
			errorType: "",
			operation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger("TestFunction")
			loggerWithError := logger.WithError(tt.err, tt.errorType, tt.operation)

			// Verify error fields were added
			if val, exists := loggerWithError.fields["error"]; !exists {
				t.Error("WithError() should add error field")
			} else if val != tt.err.Error() {
				t.Errorf("WithError() error field = %v, want %v", val, tt.err.Error())
			}

			if val, exists := loggerWithError.fields["error_type"]; !exists {
				t.Error("WithError() should add error_type field")
			} else if val != tt.errorType {
				t.Errorf("WithError() error_type field = %v, want %v", val, tt.errorType)
			}

			if val, exists := loggerWithError.fields["operation"]; !exists {
				t.Error("WithError() should add operation field")
			} else if val != tt.operation {
				t.Errorf("WithError() operation field = %v, want %v", val, tt.operation)
			}

			// Test method chaining returns same instance
			if loggerWithError != logger {
				t.Error("WithError() should return same logger instance for chaining")
			}
		})
	}
}

// setupTestLogger configures logrus for testing and returns a buffer to capture output
func setupTestLogger() *bytes.Buffer {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
	logrus.SetLevel(logrus.DebugLevel)
	return &buf
}

// TestLoggerHelper_LoggingMethods tests all logging methods
func TestLoggerHelper_LoggingMethods(t *testing.T) {
	tests := []struct {
		name        string
		method      func(*LoggerHelper, string)
		message     string
		expectLevel string
	}{
		{
			name: "Entry method",
			method: func(l *LoggerHelper, msg string) {
				l.Entry(msg)
			},
			message:     "test message",
			expectLevel: "level=debug",
		},
		{
			name: "Debug method",
			method: func(l *LoggerHelper, msg string) {
				l.Debug(msg)
			},
			message:     "debug message",
			expectLevel: "level=debug",
		},
		{
			name: "Info method",
			method: func(l *LoggerHelper, msg string) {
				l.Info(msg)
			},
			message:     "info message",
			expectLevel: "level=info",
		},
		{
			name: "Warn method",
			method: func(l *LoggerHelper, msg string) {
				l.Warn(msg)
			},
			message:     "warn message",
			expectLevel: "level=warning",
		},
		{
			name: "Error method",
			method: func(l *LoggerHelper, msg string) {
				l.Error(msg)
			},
			message:     "error message",
			expectLevel: "level=error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := setupTestLogger()
			logger := NewLogger("TestFunction")

			tt.method(logger, tt.message)

			output := buf.String()
			if !strings.Contains(output, tt.expectLevel) {
				t.Errorf("Expected log level %s in output: %s", tt.expectLevel, output)
			}

			if !strings.Contains(output, "function=TestFunction") {
				t.Errorf("Expected function field in output: %s", output)
			}

			if !strings.Contains(output, "package=crypto") {
				t.Errorf("Expected package field in output: %s", output)
			}

			// For Entry method, check for specific entry format
			if strings.Contains(tt.name, "Entry") {
				if !strings.Contains(output, "Function entry: "+tt.message) {
					t.Errorf("Expected entry format in output: %s", output)
				}
			} else if !strings.Contains(output, tt.message) {
				t.Errorf("Expected message in output: %s", output)
			}
		})
	}
}

// TestLoggerHelper_Exit tests the Exit method
func TestLoggerHelper_Exit(t *testing.T) {
	buf := setupTestLogger()
	logger := NewLogger("TestFunction")

	logger.Exit()

	output := buf.String()
	if !strings.Contains(output, "level=debug") {
		t.Errorf("Expected debug level in output: %s", output)
	}

	if !strings.Contains(output, "Function exit: TestFunction") {
		t.Errorf("Expected exit format in output: %s", output)
	}

	if !strings.Contains(output, "function=TestFunction") {
		t.Errorf("Expected function field in output: %s", output)
	}

	if !strings.Contains(output, "package=crypto") {
		t.Errorf("Expected package field in output: %s", output)
	}
}

// TestSecureFieldHash tests the SecureFieldHash function
func TestSecureFieldHash(t *testing.T) {
	tests := []struct {
		name           string
		data           []byte
		fieldName      string
		expectedSize   int
		expectPreview  string
		expectEllipsis bool
	}{
		{
			name:           "nil data",
			data:           nil,
			fieldName:      "test_field",
			expectedSize:   0,
			expectPreview:  "nil",
			expectEllipsis: false,
		},
		{
			name:           "empty data",
			data:           []byte{},
			fieldName:      "test_field",
			expectedSize:   0,
			expectPreview:  "nil",
			expectEllipsis: false,
		},
		{
			name:           "short data",
			data:           []byte{0x01, 0x02, 0x03, 0x04},
			fieldName:      "test_field",
			expectedSize:   4,
			expectPreview:  "01020304",
			expectEllipsis: false,
		},
		{
			name:           "exactly 8 bytes",
			data:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			fieldName:      "test_field",
			expectedSize:   8,
			expectPreview:  "0102030405060708",
			expectEllipsis: false,
		},
		{
			name:           "long data",
			data:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c},
			fieldName:      "test_field",
			expectedSize:   12,
			expectPreview:  "0102030405060708",
			expectEllipsis: true,
		},
		{
			name:           "different field name",
			data:           []byte{0xff, 0xee, 0xdd, 0xcc},
			fieldName:      "secret_key",
			expectedSize:   4,
			expectPreview:  "ffeeddcc",
			expectEllipsis: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := SecureFieldHash(tt.data, tt.fieldName)

			// Check size field
			sizeKey := tt.fieldName + "_size"
			if size, exists := fields[sizeKey]; !exists {
				t.Errorf("SecureFieldHash() should include %s field", sizeKey)
			} else if size != tt.expectedSize {
				t.Errorf("SecureFieldHash() %s = %v, want %v", sizeKey, size, tt.expectedSize)
			}

			// Check preview field
			previewKey := tt.fieldName + "_preview"
			if preview, exists := fields[previewKey]; !exists {
				t.Errorf("SecureFieldHash() should include %s field", previewKey)
			} else {
				previewStr, ok := preview.(string)
				if !ok {
					t.Errorf("SecureFieldHash() %s should be string", previewKey)
				} else {
					if tt.expectEllipsis {
						if !strings.HasPrefix(previewStr, tt.expectPreview) {
							t.Errorf("SecureFieldHash() %s = %v, should start with %v", previewKey, previewStr, tt.expectPreview)
						}
						if !strings.HasSuffix(previewStr, "...") {
							t.Errorf("SecureFieldHash() %s = %v, should end with '...'", previewKey, previewStr)
						}
					} else {
						if previewStr != tt.expectPreview {
							t.Errorf("SecureFieldHash() %s = %v, want %v", previewKey, previewStr, tt.expectPreview)
						}
					}
				}
			}
		})
	}
}

// TestOperationFields tests the OperationFields function
func TestOperationFields(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		status     string
		additional []logrus.Fields
		expected   map[string]interface{}
	}{
		{
			name:      "basic operation",
			operation: "encrypt",
			status:    "success",
			expected: map[string]interface{}{
				"operation": "encrypt",
				"status":    "success",
			},
		},
		{
			name:      "with single additional field",
			operation: "decrypt",
			status:    "error",
			additional: []logrus.Fields{
				{"error_code": 500},
			},
			expected: map[string]interface{}{
				"operation":  "decrypt",
				"status":     "error",
				"error_code": 500,
			},
		},
		{
			name:      "with multiple additional fields",
			operation: "key_generation",
			status:    "success",
			additional: []logrus.Fields{
				{"key_size": 32, "algorithm": "ed25519"},
				{"duration_ms": 150},
			},
			expected: map[string]interface{}{
				"operation":   "key_generation",
				"status":      "success",
				"key_size":    32,
				"algorithm":   "ed25519",
				"duration_ms": 150,
			},
		},
		{
			name:       "no additional fields",
			operation:  "validation",
			status:     "complete",
			additional: nil,
			expected: map[string]interface{}{
				"operation": "validation",
				"status":    "complete",
			},
		},
		{
			name:      "empty additional fields",
			operation: "cleanup",
			status:    "pending",
			additional: []logrus.Fields{
				{},
			},
			expected: map[string]interface{}{
				"operation": "cleanup",
				"status":    "pending",
			},
		},
		{
			name:      "overlapping field names",
			operation: "test_op",
			status:    "running",
			additional: []logrus.Fields{
				{"operation": "override_op", "custom": "value1"},
				{"status": "override_status", "custom": "value2"},
			},
			expected: map[string]interface{}{
				"operation": "override_op",     // Last additional field wins for operation
				"status":    "override_status", // Last additional field wins for status
				"custom":    "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := OperationFields(tt.operation, tt.status, tt.additional...)

			// Check that all expected fields are present with correct values
			for key, expectedValue := range tt.expected {
				if actualValue, exists := fields[key]; !exists {
					t.Errorf("OperationFields() missing field %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("OperationFields() field %s = %v, want %v", key, actualValue, expectedValue)
				}
			}

			// Check that no unexpected fields are present
			if len(fields) != len(tt.expected) {
				t.Errorf("OperationFields() returned %d fields, expected %d", len(fields), len(tt.expected))
				t.Errorf("Actual fields: %+v", fields)
				t.Errorf("Expected fields: %+v", tt.expected)
			}
		})
	}
}

// TestLoggerChaining tests method chaining
func TestLoggerChaining(t *testing.T) {
	buf := setupTestLogger()
	logger := NewLogger("ChainTest")

	// Test comprehensive method chaining
	logger.WithField("step", 1).
		WithCaller().
		WithField("data", "test").
		WithFields(logrus.Fields{
			"extra1": "value1",
			"extra2": "value2",
		}).
		WithError(errors.New("test error"), "chain_error", "chaining_test").
		Info("chained logging test")

	output := buf.String()

	// Verify all chained fields are present
	expectedFields := []string{
		"step=1",
		"data=test",
		"extra1=value1",
		"extra2=value2",
		"error=\"test error\"",
		"error_type=chain_error",
		"operation=chaining_test",
		"caller=",
		"caller_func=",
		"function=ChainTest",
		"package=crypto",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected field %s in chained output: %s", field, output)
		}
	}
}

// TestLoggerConcurrency tests logger thread safety by creating separate logger instances
func TestLoggerConcurrency(t *testing.T) {
	done := make(chan bool, 10)

	// Run multiple goroutines with separate logger instances
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Create a separate logger instance for each goroutine
			logger := NewLogger("ConcurrencyTest")

			for j := 0; j < 10; j++ {
				logger.WithField("goroutine", id).
					WithField("iteration", j).
					Debug(fmt.Sprintf("concurrent log %d-%d", id, j))
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test should complete without panics or race conditions
}

// BenchmarkNewLogger benchmarks logger creation
func BenchmarkNewLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLogger("BenchmarkFunction")
	}
}

// BenchmarkLoggerChaining benchmarks method chaining
func BenchmarkLoggerChaining(b *testing.B) {
	logger := NewLogger("BenchmarkFunction")
	testError := errors.New("benchmark error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithField("iteration", i).
			WithCaller().
			WithFields(logrus.Fields{"bench": true}).
			WithError(testError, "bench_error", "benchmark")
	}
}

// BenchmarkSecureFieldHash benchmarks secure field hashing
func BenchmarkSecureFieldHash(b *testing.B) {
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SecureFieldHash(data, "benchmark_field")
	}
}

// BenchmarkOperationFields benchmarks operation field creation
func BenchmarkOperationFields(b *testing.B) {
	additional := []logrus.Fields{
		{"key1": "value1", "key2": 42},
		{"key3": true, "key4": "value4"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = OperationFields("benchmark_op", "running", additional...)
	}
}
