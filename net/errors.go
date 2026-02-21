package net

import (
	"errors"
	"fmt"
)

// Common errors for Tox networking
var (
	// ErrInvalidToxID indicates an invalid Tox ID was provided
	ErrInvalidToxID = errors.New("invalid Tox ID")

	// ErrFriendNotFound indicates the specified friend was not found
	ErrFriendNotFound = errors.New("friend not found")

	// ErrFriendOffline indicates the friend is currently offline
	ErrFriendOffline = errors.New("friend is offline")

	// ErrConnectionClosed indicates the connection has been closed
	ErrConnectionClosed = errors.New("connection closed")

	// ErrListenerClosed indicates the listener has been closed
	ErrListenerClosed = errors.New("listener closed")

	// ErrTimeout indicates a timeout occurred
	ErrTimeout = errors.New("operation timed out")

	// ErrBufferFull indicates the internal buffer is full
	ErrBufferFull = errors.New("buffer full")

	// ErrNoPeerKey indicates no encryption key is registered for the peer
	ErrNoPeerKey = errors.New("no encryption key for peer")

	// ErrPartialWrite indicates only part of the data was written before an error occurred
	ErrPartialWrite = errors.New("partial write")
)

// ToxNetError represents a network error with additional context about the
// operation that failed. This error type wraps underlying errors to provide
// a consistent interface while preserving the original error for inspection.
//
// ToxNetError implements the error interface and supports unwrapping via
// errors.Unwrap, errors.Is, and errors.As from the standard library.
//
// Common wrapping patterns:
//
//	// Wrap a connection read error
//	if _, err := conn.Read(buf); err != nil {
//	    return &ToxNetError{Op: "read", Addr: conn.RemoteAddr().String(), Err: err}
//	}
//
//	// Wrap a dial error with NewToxNetError helper
//	return NewToxNetError("dial", toxID, ErrFriendOffline)
//
//	// Check for specific underlying errors
//	var toxErr *ToxNetError
//	if errors.As(err, &toxErr) && errors.Is(toxErr.Err, ErrTimeout) {
//	    // Handle timeout specifically
//	}
type ToxNetError struct {
	Op   string // operation that caused the error (e.g., "read", "write", "dial", "listen")
	Addr string // address if relevant (empty string if not applicable)
	Err  error  // underlying error (use errors.Is/errors.As to inspect)
}

func (e *ToxNetError) Error() string {
	if e.Addr != "" {
		return fmt.Sprintf("tox %s %s: %v", e.Op, e.Addr, e.Err)
	}
	return fmt.Sprintf("tox %s: %v", e.Op, e.Err)
}

func (e *ToxNetError) Unwrap() error {
	return e.Err
}

// NewToxNetError creates a new ToxNetError with the specified operation,
// address, and underlying error. Use this to wrap errors with contextual
// information about the network operation that failed.
//
// Example:
//
//	if err := conn.Read(buf); err != nil {
//	    return NewToxNetError("read", peerAddr, err)
//	}
func NewToxNetError(op, addr string, err error) *ToxNetError {
	return &ToxNetError{
		Op:   op,
		Addr: addr,
		Err:  err,
	}
}
