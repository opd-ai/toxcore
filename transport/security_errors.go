// Package transport provides network transport and protocol negotiation for Tox.
//
// This file defines security error classification and structured error types
// for distinguishing fatal security failures from acceptable compatibility fallbacks.
package transport

import (
	"errors"
	"fmt"
)

// SecurityErrorCategory categorizes security errors for observability and policy enforcement.
type SecurityErrorCategory int

const (
	// FatalSecurityError indicates an unrecoverable security failure that must not proceed.
	// Examples: signature verification failure, mandatory security requirement not met.
	FatalSecurityError SecurityErrorCategory = iota

	// CompatibilityWarning indicates an acceptable security downgrade that reduces
	// the security level but maintains encrypted communication.
	// Examples: negotiating to legacy protocol when peer does not support Noise-IK.
	CompatibilityWarning

	// VerificationFailure indicates a peer identity verification or trust establishment failure.
	// Examples: key change detected, signature verification failed, TOFU state mismatch.
	VerificationFailure

	// DowngradeEvent indicates a negotiated protocol downgrade.
	// Examples: fallback from Noise+Ratchet to Noise-IK, or Noise-IK to Legacy.
	DowngradeEvent
)

// String returns the human-readable name of the security error category.
func (c SecurityErrorCategory) String() string {
	switch c {
	case FatalSecurityError:
		return "FatalSecurityError"
	case CompatibilityWarning:
		return "CompatibilityWarning"
	case VerificationFailure:
		return "VerificationFailure"
	case DowngradeEvent:
		return "DowngradeEvent"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}

// SecurityError wraps an error with security classification and context.
// This type makes downgrade and verification-failure paths explicit and observable.
type SecurityError struct {
	// Category classifies the error as fatal, compatibility warning, verification failure, or downgrade.
	Category SecurityErrorCategory

	// Err is the underlying error.
	Err error

	// Event describes the security event that triggered the error.
	// Examples: "signature_verification_failed", "protocol_downgrade", "ratchet_required".
	Event string

	// Path describes the code path or component where the error occurred.
	// Examples: "version_negotiation", "noise_handshake", "async_prekey_validation".
	Path string

	// Reason provides additional context for the error.
	// Examples: "peer does not support Noise-IK", "signature verification failed".
	Reason string
}

// Error implements the error interface.
func (se *SecurityError) Error() string {
	if se.Err != nil {
		return fmt.Sprintf("[%s] %s (%s/%s): %v",
			se.Category,
			se.Event,
			se.Path,
			se.Reason,
			se.Err)
	}
	return fmt.Sprintf("[%s] %s (%s/%s)",
		se.Category,
		se.Event,
		se.Path,
		se.Reason)
}

// Unwrap returns the underlying error for error wrapping semantics.
func (se *SecurityError) Unwrap() error {
	return se.Err
}

// IsFatal returns true if this is a fatal security error.
func (se *SecurityError) IsFatal() bool {
	return se.Category == FatalSecurityError
}

// IsCompatibilityWarning returns true if this is an acceptable compatibility warning.
func (se *SecurityError) IsCompatibilityWarning() bool {
	return se.Category == CompatibilityWarning
}

// NewSecurityError creates a structured security error.
func NewSecurityError(category SecurityErrorCategory, event, path, reason string, err error) *SecurityError {
	return &SecurityError{
		Category: category,
		Err:      err,
		Event:    event,
		Path:     path,
		Reason:   reason,
	}
}

// NewFatalSecurityError creates a fatal security error that must not proceed.
func NewFatalSecurityError(event, path, reason string, err error) *SecurityError {
	return NewSecurityError(FatalSecurityError, event, path, reason, err)
}

// NewCompatibilityWarning creates a compatibility warning for acceptable fallbacks.
func NewCompatibilityWarning(event, path, reason string, err error) *SecurityError {
	return NewSecurityError(CompatibilityWarning, event, path, reason, err)
}

// NewVerificationFailure creates a verification failure error.
func NewVerificationFailure(event, path, reason string, err error) *SecurityError {
	return NewSecurityError(VerificationFailure, event, path, reason, err)
}

// NewDowngradeEvent creates a downgrade event error.
func NewDowngradeEvent(event, path, reason string, err error) *SecurityError {
	return NewSecurityError(DowngradeEvent, event, path, reason, err)
}

// AsSecurityError converts an error to a SecurityError if it is one.
// Returns the SecurityError and true if the conversion succeeds, nil and false otherwise.
func AsSecurityError(err error) (*SecurityError, bool) {
	var se *SecurityError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

// Predefined fatal security errors for common scenarios.
var (
	// ErrSignatureVerificationFailed indicates that a signature verification failed during negotiation.
	ErrSignatureVerificationFailed = NewFatalSecurityError(
		"signature_verification_failed",
		"version_negotiation",
		"signature verification failed",
		errors.New("signature verification failed"),
	)

	// ErrMandatorySecurityRequirementNotMet indicates that a mandatory security requirement was not met.
	ErrMandatorySecurityRequirementNotMet = NewFatalSecurityError(
		"mandatory_security_requirement_not_met",
		"protocol_negotiation",
		"mandatory security requirement not met",
		errors.New("mandatory security requirement not met"),
	)

	// ErrVersionCommitmentMismatch indicates a version commitment mismatch.
	ErrVersionCommitmentMismatch = NewFatalSecurityError(
		"version_commitment_mismatch",
		"version_negotiation",
		"version commitment mismatch detected",
		errors.New("version commitment mismatch"),
	)

	// ErrNoCommonProtocolVersion indicates that no common protocol version is supported.
	ErrNoCommonProtocolVersion = NewFatalSecurityError(
		"no_common_protocol_version",
		"version_negotiation",
		"no common protocol version supported",
		errors.New("no common protocol version"),
	)
)

// Predefined compatibility warnings for common scenarios.
var (
	// WarnFallbackToLegacy indicates a fallback to legacy protocol.
	WarnFallbackToLegacy = NewCompatibilityWarning(
		"protocol_fallback",
		"version_negotiation",
		"falling back to legacy protocol",
		errors.New("legacy protocol fallback"),
	)

	// WarnRatchetNotSupported indicates that ratcheting is not supported by peer.
	WarnRatchetNotSupported = NewCompatibilityWarning(
		"feature_not_supported",
		"ratchet_negotiation",
		"peer does not support ratcheting",
		errors.New("ratchet not supported"),
	)
)
