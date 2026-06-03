package transport

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityErrorCategoryString verifies category string representations.
func TestSecurityErrorCategoryString(t *testing.T) {
	tests := map[SecurityErrorCategory]string{
		FatalSecurityError:   "FatalSecurityError",
		CompatibilityWarning: "CompatibilityWarning",
		VerificationFailure:  "VerificationFailure",
		DowngradeEvent:       "DowngradeEvent",
	}

	for category, expected := range tests {
		t.Run(expected, func(t *testing.T) {
			assert.Equal(t, expected, category.String())
		})
	}
}

// TestUnknownSecurityErrorCategory verifies unknown category string representation.
func TestUnknownSecurityErrorCategory(t *testing.T) {
	unknown := SecurityErrorCategory(999)
	assert.Equal(t, "Unknown(999)", unknown.String())
}

// TestSecurityErrorError verifies error message formatting.
func TestSecurityErrorError(t *testing.T) {
	tests := []struct {
		name     string
		se       *SecurityError
		contains string
	}{
		{
			name: "with underlying error",
			se: NewSecurityError(
				FatalSecurityError,
				"test_event",
				"test_path",
				"test reason",
				errors.New("underlying error"),
			),
			contains: "[FatalSecurityError] test_event (test_path/test reason): underlying error",
		},
		{
			name: "without underlying error",
			se: NewSecurityError(
				CompatibilityWarning,
				"event",
				"path",
				"reason",
				nil,
			),
			contains: "[CompatibilityWarning] event (path/reason)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.se.Error(), tt.contains)
		})
	}
}

// TestSecurityErrorUnwrap verifies error unwrapping.
func TestSecurityErrorUnwrap(t *testing.T) {
	underlyingErr := errors.New("underlying")
	se := NewSecurityError(FatalSecurityError, "event", "path", "reason", underlyingErr)

	assert.Equal(t, underlyingErr, se.Unwrap())
	assert.True(t, errors.Is(se, underlyingErr))
}

// TestSecurityErrorIsFatal verifies fatal classification.
func TestSecurityErrorIsFatal(t *testing.T) {
	tests := []struct {
		name     string
		se       *SecurityError
		expected bool
	}{
		{
			name:     "fatal error",
			se:       NewFatalSecurityError("event", "path", "reason", nil),
			expected: true,
		},
		{
			name:     "compatibility warning",
			se:       NewCompatibilityWarning("event", "path", "reason", nil),
			expected: false,
		},
		{
			name:     "verification failure",
			se:       NewVerificationFailure("event", "path", "reason", nil),
			expected: false,
		},
		{
			name:     "downgrade event",
			se:       NewDowngradeEvent("event", "path", "reason", nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.se.IsFatal())
		})
	}
}

// TestSecurityErrorIsCompatibilityWarning verifies compatibility warning classification.
func TestSecurityErrorIsCompatibilityWarning(t *testing.T) {
	tests := []struct {
		name     string
		se       *SecurityError
		expected bool
	}{
		{
			name:     "fatal error",
			se:       NewFatalSecurityError("event", "path", "reason", nil),
			expected: false,
		},
		{
			name:     "compatibility warning",
			se:       NewCompatibilityWarning("event", "path", "reason", nil),
			expected: true,
		},
		{
			name:     "verification failure",
			se:       NewVerificationFailure("event", "path", "reason", nil),
			expected: false,
		},
		{
			name:     "downgrade event",
			se:       NewDowngradeEvent("event", "path", "reason", nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.se.IsCompatibilityWarning())
		})
	}
}

// TestNewSecurityError verifies structured error creation.
func TestNewSecurityError(t *testing.T) {
	underlyingErr := errors.New("underlying")
	se := NewSecurityError(
		FatalSecurityError,
		"test_event",
		"test_path",
		"test_reason",
		underlyingErr,
	)

	assert.Equal(t, FatalSecurityError, se.Category)
	assert.Equal(t, "test_event", se.Event)
	assert.Equal(t, "test_path", se.Path)
	assert.Equal(t, "test_reason", se.Reason)
	assert.True(t, errors.Is(se, underlyingErr))
}

// TestAsSecurityError verifies error conversion.
func TestAsSecurityError(t *testing.T) {
	se := NewFatalSecurityError("event", "path", "reason", nil)

	retrieved, ok := AsSecurityError(se)
	require.True(t, ok)
	assert.Equal(t, se.Category, retrieved.Category)
	assert.Equal(t, se.Event, retrieved.Event)

	// Wrapped in another error
	wrapped := fmt.Errorf("wrapped: %w", se)
	retrieved2, ok2 := AsSecurityError(wrapped)
	require.True(t, ok2)
	assert.Equal(t, se.Category, retrieved2.Category)

	// Non-SecurityError
	nonSE := errors.New("not a security error")
	_, ok3 := AsSecurityError(nonSE)
	assert.False(t, ok3)
}

// TestPredefinedFatalErrors verifies predefined fatal errors.
func TestPredefinedFatalErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   *SecurityError
		event string
	}{
		{
			name:  "signature verification failed",
			err:   ErrSignatureVerificationFailed,
			event: "signature_verification_failed",
		},
		{
			name:  "mandatory security requirement not met",
			err:   ErrMandatorySecurityRequirementNotMet,
			event: "mandatory_security_requirement_not_met",
		},
		{
			name:  "version commitment mismatch",
			err:   ErrVersionCommitmentMismatch,
			event: "version_commitment_mismatch",
		},
		{
			name:  "no common protocol version",
			err:   ErrNoCommonProtocolVersion,
			event: "no_common_protocol_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.err.IsFatal())
			assert.Equal(t, tt.event, tt.err.Event)
			assert.Equal(t, FatalSecurityError, tt.err.Category)
		})
	}
}

// TestPredefinedCompatibilityWarnings verifies predefined compatibility warnings.
func TestPredefinedCompatibilityWarnings(t *testing.T) {
	tests := []struct {
		name  string
		warn  *SecurityError
		event string
	}{
		{
			name:  "fallback to legacy",
			warn:  WarnFallbackToLegacy,
			event: "protocol_fallback",
		},
		{
			name:  "ratchet not supported",
			warn:  WarnRatchetNotSupported,
			event: "feature_not_supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.warn.IsCompatibilityWarning())
			assert.Equal(t, tt.event, tt.warn.Event)
			assert.Equal(t, CompatibilityWarning, tt.warn.Category)
		})
	}
}

// TestSecurityErrorContextFields verifies all context fields are accessible.
func TestSecurityErrorContextFields(t *testing.T) {
	se := NewSecurityError(
		VerificationFailure,
		"peer_key_changed",
		"friend_connection",
		"peer public key changed - possible MITM attack",
		errors.New("key mismatch"),
	)

	assert.Equal(t, VerificationFailure, se.Category)
	assert.Equal(t, "peer_key_changed", se.Event)
	assert.Equal(t, "friend_connection", se.Path)
	assert.Equal(t, "peer public key changed - possible MITM attack", se.Reason)
	assert.NotNil(t, se.Err)
	assert.Equal(t, "key mismatch", se.Err.Error())
}

// TestDowngradeEventCategory verifies downgrade event classification.
func TestDowngradeEventCategory(t *testing.T) {
	se := NewDowngradeEvent(
		"negotiate_fallback",
		"version_negotiation",
		"peer does not support Noise-IK",
		nil,
	)

	assert.Equal(t, DowngradeEvent, se.Category)
	assert.False(t, se.IsFatal())
	assert.False(t, se.IsCompatibilityWarning()) // Downgrade is distinct from compatibility warning
}

// BenchmarkSecurityErrorCreation measures the performance of creating security errors.
func BenchmarkSecurityErrorCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewSecurityError(
			FatalSecurityError,
			"event",
			"path",
			"reason",
			errors.New("error"),
		)
	}
}

// BenchmarkSecurityErrorClassification measures the performance of error classification.
func BenchmarkSecurityErrorClassification(b *testing.B) {
	se := NewFatalSecurityError("event", "path", "reason", nil)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = se.IsFatal()
		_ = se.IsCompatibilityWarning()
	}
}
