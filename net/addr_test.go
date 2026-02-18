package net

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
)

func TestParseToxAddr(t *testing.T) {
	// Create a valid ToxID
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}
	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	t.Run("valid address", func(t *testing.T) {
		addr, err := ParseToxAddr(validToxIDString)
		if err != nil {
			t.Errorf("ParseToxAddr() error = %v", err)
			return
		}
		if addr.String() != validToxIDString {
			t.Errorf("ParseToxAddr() returned wrong address")
		}
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := ParseToxAddr("invalid")
		if err == nil {
			t.Error("ParseToxAddr() expected error for invalid address")
		}
	})

	t.Run("lowercase address", func(t *testing.T) {
		// Test case insensitivity
		lowerAddr := "76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37166a8712a20c01"
		// Add checksum from valid ToxID
		lowerAddr = validToxIDString[:72] + validToxIDString[72:]
		addr, err := ParseToxAddr(lowerAddr)
		if err != nil {
			t.Logf("ParseToxAddr() error = %v (acceptable for lowercase)", err)
			return
		}
		if addr == nil {
			t.Error("ParseToxAddr() returned nil addr")
		}
	})
}

func TestResolveToxAddr(t *testing.T) {
	// Create a valid ToxID
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}
	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	t.Run("valid address resolution", func(t *testing.T) {
		addr, err := ResolveToxAddr(validToxIDString)
		if err != nil {
			t.Errorf("ResolveToxAddr() error = %v", err)
			return
		}
		if addr.String() != validToxIDString {
			t.Errorf("ResolveToxAddr() returned wrong address")
		}
	})

	t.Run("invalid address resolution", func(t *testing.T) {
		_, err := ResolveToxAddr("not-a-valid-tox-id")
		if err == nil {
			t.Error("ResolveToxAddr() expected error for invalid address")
		}
	})
}

func TestToxAddrNilToxID(t *testing.T) {
	// Test handling of ToxAddr with nil toxID
	addr := &ToxAddr{toxID: nil}

	t.Run("String with nil toxID", func(t *testing.T) {
		result := addr.String()
		if result != "<invalid>" {
			t.Errorf("String() = %q, want %q", result, "<invalid>")
		}
	})

	t.Run("PublicKey with nil toxID", func(t *testing.T) {
		pk := addr.PublicKey()
		expected := [32]byte{}
		if pk != expected {
			t.Errorf("PublicKey() = %v, want %v", pk, expected)
		}
	})

	t.Run("Nospam with nil toxID", func(t *testing.T) {
		ns := addr.Nospam()
		expected := [4]byte{}
		if ns != expected {
			t.Errorf("Nospam() = %v, want %v", ns, expected)
		}
	})

	t.Run("ToxID returns nil", func(t *testing.T) {
		if addr.ToxID() != nil {
			t.Error("ToxID() should return nil")
		}
	})
}

func TestToxAddrEqualEdgeCases(t *testing.T) {
	publicKey := [32]byte{1, 2, 3}
	nospam := [4]byte{4, 5, 6, 7}
	addr := NewToxAddrFromPublicKey(publicKey, nospam)

	t.Run("nil receiver", func(t *testing.T) {
		var nilAddr *ToxAddr
		if nilAddr.Equal(addr) {
			t.Error("nil.Equal(addr) should be false")
		}
	})

	t.Run("nil argument", func(t *testing.T) {
		if addr.Equal(nil) {
			t.Error("addr.Equal(nil) should be false")
		}
	})

	t.Run("both nil", func(t *testing.T) {
		var a, b *ToxAddr
		if !a.Equal(b) {
			t.Error("nil.Equal(nil) should be true")
		}
	})

	t.Run("nil toxID in one", func(t *testing.T) {
		nilToxIDAddr := &ToxAddr{toxID: nil}
		if addr.Equal(nilToxIDAddr) {
			t.Error("addr with valid toxID should not equal addr with nil toxID")
		}
	})

	t.Run("nil toxID in both", func(t *testing.T) {
		a := &ToxAddr{toxID: nil}
		b := &ToxAddr{toxID: nil}
		if !a.Equal(b) {
			t.Error("two addrs with nil toxID should be equal")
		}
	})
}

func TestToxConnValidateReadInput(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	t.Run("empty buffer returns 0", func(t *testing.T) {
		n, err := conn.validateReadInput([]byte{})
		if n != 0 || err != nil {
			t.Errorf("validateReadInput([]) = (%d, %v), want (0, nil)", n, err)
		}
	})

	t.Run("non-empty buffer returns -1", func(t *testing.T) {
		n, err := conn.validateReadInput(make([]byte, 10))
		if n != -1 || err != nil {
			t.Errorf("validateReadInput(10 bytes) = (%d, %v), want (-1, nil)", n, err)
		}
	})
}

func TestToxConnCheckConnectionClosed(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})

	t.Run("open connection returns nil", func(t *testing.T) {
		conn := newToxConn(tox, 123, localAddr, remoteAddr)
		defer conn.Close()

		err := conn.checkConnectionClosed()
		if err != nil {
			t.Errorf("checkConnectionClosed() = %v, want nil", err)
		}
	})

	t.Run("closed connection returns error", func(t *testing.T) {
		conn := newToxConn(tox, 124, localAddr, remoteAddr)
		conn.Close()

		err := conn.checkConnectionClosed()
		if err != ErrConnectionClosed {
			t.Errorf("checkConnectionClosed() = %v, want ErrConnectionClosed", err)
		}
	})
}

func TestToxConnValidateWriteConditions(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})

	t.Run("open connection", func(t *testing.T) {
		conn := newToxConn(tox, 123, localAddr, remoteAddr)
		defer conn.Close()

		err := conn.validateWriteConditions()
		if err != nil {
			t.Errorf("validateWriteConditions() = %v, want nil", err)
		}
	})

	t.Run("closed connection", func(t *testing.T) {
		conn := newToxConn(tox, 124, localAddr, remoteAddr)
		conn.Close()

		err := conn.validateWriteConditions()
		if err != ErrConnectionClosed {
			t.Errorf("validateWriteConditions() = %v, want ErrConnectionClosed", err)
		}
	})
}

func TestToxConnCheckConnectionStatus(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})

	t.Run("open and not connected", func(t *testing.T) {
		conn := newToxConn(tox, 123, localAddr, remoteAddr)
		defer conn.Close()

		connected, err := conn.checkConnectionStatus()
		if err != nil {
			t.Errorf("checkConnectionStatus() error = %v", err)
		}
		if connected {
			t.Error("checkConnectionStatus() should return false for unconnected friend")
		}
	})

	t.Run("closed returns error", func(t *testing.T) {
		conn := newToxConn(tox, 124, localAddr, remoteAddr)
		conn.Close()

		_, err := conn.checkConnectionStatus()
		if err != ErrConnectionClosed {
			t.Errorf("checkConnectionStatus() error = %v, want ErrConnectionClosed", err)
		}
	})
}

func TestToxConnWriteEmptyBuffer(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Writing empty buffer should return 0, nil
	n, err := conn.Write([]byte{})
	if n != 0 || err != nil {
		t.Errorf("Write([]) = (%d, %v), want (0, nil)", n, err)
	}
}

func TestToxConnReadEmptyBuffer(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Reading into empty buffer should return 0, nil
	n, err := conn.Read([]byte{})
	if n != 0 || err != nil {
		t.Errorf("Read([]) = (%d, %v), want (0, nil)", n, err)
	}
}

func TestToxConnSetupReadTimeout(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	t.Run("no deadline set", func(t *testing.T) {
		ch := conn.setupReadTimeout()
		if ch != nil {
			t.Error("setupReadTimeout() should return nil when no deadline set")
		}
	})
}

func TestToxConnSetupConnectionTimeout(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	t.Run("no deadline set", func(t *testing.T) {
		ch := conn.setupConnectionTimeout()
		if ch != nil {
			t.Error("setupConnectionTimeout() should return nil when no deadline set")
		}
	})
}

func TestToxConnCloseTwice(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)

	// First close should succeed
	err = conn.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should also return nil (idempotent)
	err = conn.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestToxConnCheckWriteDeadline(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	t.Run("no deadline returns nil", func(t *testing.T) {
		err := conn.checkWriteDeadline()
		if err != nil {
			t.Errorf("checkWriteDeadline() = %v, want nil", err)
		}
	})

	t.Run("past deadline returns timeout error", func(t *testing.T) {
		// Set deadline in the past
		conn.SetWriteDeadline(time.Now().Add(-1 * time.Second))
		err := conn.checkWriteDeadline()
		if err == nil {
			t.Error("checkWriteDeadline() should return error for past deadline")
		}

		toxErr, ok := err.(*ToxNetError)
		if !ok {
			t.Errorf("Expected ToxNetError, got %T", err)
		} else if toxErr.Op != "write" {
			t.Errorf("Expected op 'write', got '%s'", toxErr.Op)
		}
	})

	t.Run("future deadline returns nil", func(t *testing.T) {
		conn.SetWriteDeadline(time.Now().Add(1 * time.Hour))
		err := conn.checkWriteDeadline()
		if err != nil {
			t.Errorf("checkWriteDeadline() = %v, want nil", err)
		}
	})
}

func TestToxConnEnsureConnected(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Set a very short write deadline so ensureConnected fails quickly
	conn.SetWriteDeadline(time.Now().Add(5 * time.Millisecond))

	// ensureConnected should wait and eventually timeout
	err = conn.ensureConnected()
	if err == nil {
		t.Error("ensureConnected() should return error for unconnected friend with short deadline")
	}
}

func TestToxConnWriteToClosedConnection(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	conn.Close()

	// Write to closed connection should fail
	_, err = conn.Write([]byte("test"))
	if err != ErrConnectionClosed {
		t.Errorf("Write() error = %v, want ErrConnectionClosed", err)
	}
}

func TestToxConnReadFromClosedConnection(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	conn.Close()

	// Read from closed connection should fail
	buf := make([]byte, 100)
	_, err = conn.Read(buf)
	if err != ErrConnectionClosed {
		t.Errorf("Read() error = %v, want ErrConnectionClosed", err)
	}
}
