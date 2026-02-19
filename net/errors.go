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

// ToxNetError represents an error with additional context
type ToxNetError struct {
	Op   string // operation that caused the error
	Addr string // address if relevant
	Err  error  // underlying error
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

// newToxNetError creates a new ToxNetError
func newToxNetError(op, addr string, err error) *ToxNetError {
	return &ToxNetError{
		Op:   op,
		Addr: addr,
		Err:  err,
	}
}
