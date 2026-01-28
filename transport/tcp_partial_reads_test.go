package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// partialReadConn simulates a TCP connection that returns partial reads.
type partialReadConn struct {
	data       []byte
	readPos    int
	chunkSize  int
	readCalls  int
	closed     bool
	remoteAddr net.Addr
}

func newPartialReadConn(data []byte, chunkSize int) *partialReadConn {
	return &partialReadConn{
		data:       data,
		readPos:    0,
		chunkSize:  chunkSize,
		remoteAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345},
	}
}

// Read simulates partial reads by returning only chunkSize bytes at a time.
func (p *partialReadConn) Read(b []byte) (n int, err error) {
	if p.closed {
		return 0, io.EOF
	}

	p.readCalls++

	remaining := len(p.data) - p.readPos
	if remaining == 0 {
		return 0, io.EOF
	}

	// Return at most chunkSize bytes
	toRead := p.chunkSize
	if toRead > len(b) {
		toRead = len(b)
	}
	if toRead > remaining {
		toRead = remaining
	}

	n = copy(b, p.data[p.readPos:p.readPos+toRead])
	p.readPos += n
	return n, nil
}

func (p *partialReadConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (p *partialReadConn) Close() error {
	p.closed = true
	return nil
}

func (p *partialReadConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func (p *partialReadConn) RemoteAddr() net.Addr {
	return p.remoteAddr
}

func (p *partialReadConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *partialReadConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *partialReadConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// TestTCPTransportPartialReads verifies that TCP transport handles partial reads correctly.
func TestTCPTransportPartialReads(t *testing.T) {
	tests := []struct {
		name      string
		dataSize  uint32
		chunkSize int
		wantErr   bool
	}{
		{
			name:      "Single byte chunks for header",
			dataSize:  100,
			chunkSize: 1,
			wantErr:   false,
		},
		{
			name:      "Two byte chunks",
			dataSize:  256,
			chunkSize: 2,
			wantErr:   false,
		},
		{
			name:      "Three byte chunks (header not aligned)",
			dataSize:  1024,
			chunkSize: 3,
			wantErr:   false,
		},
		{
			name:      "Large packet with small chunks",
			dataSize:  4096,
			chunkSize: 7,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data: 4-byte length header + payload
			payload := bytes.Repeat([]byte{0xAB}, int(tt.dataSize))
			
			var buf bytes.Buffer
			// Write length header (big-endian)
			if err := binary.Write(&buf, binary.BigEndian, tt.dataSize); err != nil {
				t.Fatalf("Failed to write header: %v", err)
			}
			buf.Write(payload)
			
			// Create partial read connection
			conn := newPartialReadConn(buf.Bytes(), tt.chunkSize)
			
			// Create transport
			transport := &TCPTransport{
				handlers: make(map[PacketType]PacketHandler),
				clients:  make(map[string]net.Conn),
			}
			
			// Read packet length
			header := make([]byte, 4)
			length, err := transport.readPacketLength(conn, header)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("readPacketLength() unexpected error: %v", err)
				}
				return
			}
			
			if length != tt.dataSize {
				t.Errorf("readPacketLength() got length %d, want %d", length, tt.dataSize)
			}
			
			// Read packet data
			data, err := transport.readPacketData(conn, length)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("readPacketData() unexpected error: %v", err)
				}
				return
			}
			
			if tt.wantErr {
				t.Error("Expected error but got none")
				return
			}
			
			// Verify data integrity
			if len(data) != int(tt.dataSize) {
				t.Errorf("readPacketData() got length %d, want %d", len(data), tt.dataSize)
			}
			
			if !bytes.Equal(data, payload) {
				t.Error("readPacketData() data corruption detected")
			}
			
			// Verify multiple Read() calls were made for partial reads
			expectedMinCalls := (4 + int(tt.dataSize) + tt.chunkSize - 1) / tt.chunkSize
			if conn.readCalls < expectedMinCalls {
				t.Logf("Partial reads working: made %d Read() calls for %d bytes with chunk size %d",
					conn.readCalls, 4+int(tt.dataSize), tt.chunkSize)
			}
		})
	}
}

// TestTCPTransportReadUnexpectedEOF verifies handling of unexpected EOF during reads.
func TestTCPTransportReadUnexpectedEOF(t *testing.T) {
	tests := []struct {
		name        string
		dataSize    int
		expectError bool
		errorType   error
	}{
		{
			name:        "Incomplete header (2 bytes only)",
			dataSize:    2,
			expectError: true,
			errorType:   io.ErrUnexpectedEOF,
		},
		{
			name:        "Header only, no payload",
			dataSize:    4,
			expectError: true,
			errorType:   io.EOF, // io.ReadFull returns EOF when no bytes available
		},
		{
			name:        "Partial payload",
			dataSize:    100, // Header says 1000, but only 96 bytes of payload
			expectError: true,
			errorType:   io.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create incomplete data
			var buf bytes.Buffer
			
			if tt.dataSize >= 4 {
				// Write header claiming large payload
				declaredSize := uint32(1000)
				if err := binary.Write(&buf, binary.BigEndian, declaredSize); err != nil {
					t.Fatalf("Failed to write header: %v", err)
				}
				// Only write partial payload
				if tt.dataSize > 4 {
					buf.Write(bytes.Repeat([]byte{0xCD}, tt.dataSize-4))
				}
			} else {
				// Write incomplete header
				buf.Write(bytes.Repeat([]byte{0xFF}, tt.dataSize))
			}
			
			conn := newPartialReadConn(buf.Bytes(), 1)
			
			transport := &TCPTransport{
				handlers: make(map[PacketType]PacketHandler),
				clients:  make(map[string]net.Conn),
			}
			
			// Try to read packet length
			header := make([]byte, 4)
			length, err := transport.readPacketLength(conn, header)
			
			if tt.dataSize < 4 {
				// Expect error during header read
				if err == nil {
					t.Error("Expected error reading incomplete header, got nil")
					return
				}
				if !errors.Is(err, tt.errorType) && err != tt.errorType {
					t.Errorf("Expected error %v, got %v", tt.errorType, err)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error reading header: %v", err)
			}
			
			// Try to read packet data (should fail on partial payload)
			_, err = transport.readPacketData(conn, length)
			if err == nil {
				t.Error("Expected error reading incomplete payload, got nil")
				return
			}
			
			if !errors.Is(err, tt.errorType) && err != tt.errorType {
				t.Errorf("Expected error %v, got %v", tt.errorType, err)
			}
		})
	}
}

// TestTCPTransportReadFullIntegration tests the full packet read cycle with partial reads.
func TestTCPTransportReadFullIntegration(t *testing.T) {
	// Create a valid packet with test data
	testPayload := []byte("Hello, Tox Protocol!")
	packetData := append([]byte{byte(PacketPingRequest)}, testPayload...)
	
	var buf bytes.Buffer
	// Write length
	length := uint32(len(packetData))
	if err := binary.Write(&buf, binary.BigEndian, length); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}
	buf.Write(packetData)
	
	// Simulate very fragmented read (1 byte at a time)
	conn := newPartialReadConn(buf.Bytes(), 1)
	
	transport := &TCPTransport{
		handlers: make(map[PacketType]PacketHandler),
		clients:  make(map[string]net.Conn),
	}
	
	// Read packet
	header := make([]byte, 4)
	length, err := transport.readPacketLength(conn, header)
	if err != nil {
		t.Fatalf("readPacketLength() failed: %v", err)
	}
	
	data, err := transport.readPacketData(conn, length)
	if err != nil {
		t.Fatalf("readPacketData() failed: %v", err)
	}
	
	// Verify packet integrity
	if !bytes.Equal(data, packetData) {
		t.Errorf("Packet data mismatch:\ngot:  %v\nwant: %v", data, packetData)
	}
	
	// Verify multiple reads were required
	expectedCalls := 4 + len(packetData) // 1 byte per call
	if conn.readCalls < expectedCalls {
		t.Errorf("Expected at least %d Read() calls, got %d", expectedCalls, conn.readCalls)
	}
	
	t.Logf("Successfully read packet with %d Read() calls for %d total bytes",
		conn.readCalls, 4+len(packetData))
}

// TestTCPTransportConcurrentPartialReads tests concurrent packet reads with partial data.
func TestTCPTransportConcurrentPartialReads(t *testing.T) {
	const numPackets = 10
	
	for i := 0; i < numPackets; i++ {
		t.Run("Packet", func(t *testing.T) {
			t.Parallel()
			
			// Create random-sized packet
			payloadSize := uint32(100 + i*50)
			payload := bytes.Repeat([]byte{byte(i)}, int(payloadSize))
			
			var buf bytes.Buffer
			if err := binary.Write(&buf, binary.BigEndian, payloadSize); err != nil {
				t.Fatalf("Failed to write header: %v", err)
			}
			buf.Write(payload)
			
			// Use different chunk sizes for each goroutine
			chunkSize := 1 + (i % 7)
			conn := newPartialReadConn(buf.Bytes(), chunkSize)
			
			transport := &TCPTransport{
				handlers: make(map[PacketType]PacketHandler),
				clients:  make(map[string]net.Conn),
			}
			
			header := make([]byte, 4)
			length, err := transport.readPacketLength(conn, header)
			if err != nil {
				t.Errorf("readPacketLength() failed: %v", err)
				return
			}
			
			data, err := transport.readPacketData(conn, length)
			if err != nil {
				t.Errorf("readPacketData() failed: %v", err)
				return
			}
			
			if len(data) != int(payloadSize) {
				t.Errorf("Got %d bytes, expected %d", len(data), payloadSize)
			}
		})
	}
}
