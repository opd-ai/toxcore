package crypto

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// LoggerHelper provides standardized logging functionality for the crypto package
type LoggerHelper struct {
	function string
	pkg      string
	fields   logrus.Fields
}

// NewLogger creates a new logger helper with standardized fields
func NewLogger(function string) *LoggerHelper {
	return &LoggerHelper{
		function: function,
		pkg:      "crypto",
		fields: logrus.Fields{
			"function": function,
			"package":  "crypto",
		},
	}
}

// WithCaller adds caller information to the logger
func (l *LoggerHelper) WithCaller() *LoggerHelper {
	if pc, file, line, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			funcName := fn.Name()
			if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
				funcName = funcName[lastSlash+1:]
			}
			l.fields["caller"] = fmt.Sprintf("%s:%d", file, line)
			l.fields["caller_func"] = funcName
		}
	}
	return l
}

// WithField adds a custom field to the logger
func (l *LoggerHelper) WithField(key string, value interface{}) *LoggerHelper {
	l.fields[key] = value
	return l
}

// WithFields adds multiple custom fields to the logger
func (l *LoggerHelper) WithFields(fields logrus.Fields) *LoggerHelper {
	for k, v := range fields {
		l.fields[k] = v
	}
	return l
}

// WithError adds error information to the logger
func (l *LoggerHelper) WithError(err error, errorType, operation string) *LoggerHelper {
	l.fields["error"] = err.Error()
	l.fields["error_type"] = errorType
	l.fields["operation"] = operation
	return l
}

// Entry logs function entry
func (l *LoggerHelper) Entry(message string) {
	logrus.WithFields(l.fields).Debug(fmt.Sprintf("Function entry: %s", message))
}

// Exit logs function exit
func (l *LoggerHelper) Exit() {
	logrus.WithFields(l.fields).Debug(fmt.Sprintf("Function exit: %s", l.function))
}

// Debug logs a debug message
func (l *LoggerHelper) Debug(message string) {
	logrus.WithFields(l.fields).Debug(message)
}

// Info logs an info message
func (l *LoggerHelper) Info(message string) {
	logrus.WithFields(l.fields).Info(message)
}

// Warn logs a warning message
func (l *LoggerHelper) Warn(message string) {
	logrus.WithFields(l.fields).Warn(message)
}

// Error logs an error message
func (l *LoggerHelper) Error(message string) {
	logrus.WithFields(l.fields).Error(message)
}

// Fatal logs a fatal message
func (l *LoggerHelper) Fatal(message string) {
	logrus.WithFields(l.fields).Fatal(message)
}

// SecureFieldHash creates a secure hash preview of sensitive data for logging
// This shows only the first 8 bytes of sensitive data for debugging purposes
func SecureFieldHash(data []byte, name string) logrus.Fields {
	preview := "nil"
	if len(data) > 0 {
		previewLen := 8
		if len(data) < previewLen {
			previewLen = len(data)
		}
		preview = fmt.Sprintf("%x", data[:previewLen])
		if len(data) > previewLen {
			preview += "..."
		}
	}

	return logrus.Fields{
		name + "_preview": preview,
		name + "_size":    len(data),
	}
}

// OperationFields creates standardized operation logging fields
func OperationFields(operation, status string, additional ...logrus.Fields) logrus.Fields {
	fields := logrus.Fields{
		"operation": operation,
		"status":    status,
	}

	for _, extra := range additional {
		for k, v := range extra {
			fields[k] = v
		}
	}

	return fields
}
