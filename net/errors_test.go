package net

import (
	"errors"
	"testing"
)

func TestToxNetError(t *testing.T) {
	t.Run("Error with address", func(t *testing.T) {
		err := &ToxNetError{
			Op:   "connect",
			Addr: "test-address",
			Err:  ErrConnectionClosed,
		}
		expected := "tox connect test-address: connection closed"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Error without address", func(t *testing.T) {
		err := &ToxNetError{
			Op:  "write",
			Err: ErrTimeout,
		}
		expected := "tox write: operation timed out"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlying := ErrBufferFull
		err := &ToxNetError{
			Op:  "read",
			Err: underlying,
		}
		if err.Unwrap() != underlying {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), underlying)
		}

		// Test with errors.Is
		if !errors.Is(err, underlying) {
			t.Error("errors.Is should return true for underlying error")
		}
	})
}

func TestNewToxNetError(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		addr    string
		err     error
		wantOp  string
		wantMsg string
	}{
		{
			name:    "with address",
			op:      "dial",
			addr:    "76518406F6A9F221...",
			err:     ErrInvalidToxID,
			wantOp:  "dial",
			wantMsg: "tox dial 76518406F6A9F221...: invalid Tox ID",
		},
		{
			name:    "without address",
			op:      "accept",
			addr:    "",
			err:     ErrListenerClosed,
			wantOp:  "accept",
			wantMsg: "tox accept: listener closed",
		},
		{
			name:    "custom error",
			op:      "parse",
			addr:    "invalid",
			err:     errors.New("custom error"),
			wantOp:  "parse",
			wantMsg: "tox parse invalid: custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newToxNetError(tt.op, tt.addr, tt.err)
			if err.Op != tt.wantOp {
				t.Errorf("Op = %q, want %q", err.Op, tt.wantOp)
			}
			if err.Addr != tt.addr {
				t.Errorf("Addr = %q, want %q", err.Addr, tt.addr)
			}
			if err.Err != tt.err {
				t.Errorf("Err = %v, want %v", err.Err, tt.err)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestErrorVariables(t *testing.T) {
	// Ensure error variables are properly defined and unique
	errorVars := map[string]error{
		"ErrInvalidToxID":     ErrInvalidToxID,
		"ErrFriendNotFound":   ErrFriendNotFound,
		"ErrFriendOffline":    ErrFriendOffline,
		"ErrConnectionClosed": ErrConnectionClosed,
		"ErrListenerClosed":   ErrListenerClosed,
		"ErrTimeout":          ErrTimeout,
		"ErrBufferFull":       ErrBufferFull,
	}

	for name, err := range errorVars {
		if err == nil {
			t.Errorf("%s is nil", name)
		}
		if err.Error() == "" {
			t.Errorf("%s has empty message", name)
		}
	}

	// Verify each error has a unique message
	seen := make(map[string]string)
	for name, err := range errorVars {
		msg := err.Error()
		if prevName, exists := seen[msg]; exists {
			t.Errorf("%s and %s have the same error message: %q", name, prevName, msg)
		}
		seen[msg] = name
	}
}
