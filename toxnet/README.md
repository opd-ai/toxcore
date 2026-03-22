# Tox Network Package

This package provides Go standard library networking interfaces for the Tox protocol. It implements `net.Conn`, `net.Listener`, and `net.Addr` interfaces to allow Tox-based encrypted peer-to-peer communication to work seamlessly with existing Go networking code.

## Features

- **ToxAddr**: Implementation of `net.Addr` for Tox IDs
- **ToxConn**: Implementation of `net.Conn` for peer-to-peer connections  
- **ToxListener**: Implementation of `net.Listener` for accepting connections
- **Dial/Listen functions**: For establishing connections
- **Thread-safe**: Safe for concurrent use
- **Timeout support**: Deadline management for read/write operations
- **Message chunking**: Automatic handling of large messages over Tox's message limits

## Quick Start

```go
package main

import (
    "fmt"
    "io"
    "log"
    
    "github.com/opd-ai/toxcore"
    toxnet "github.com/opd-ai/toxcore/net"
)

func main() {
    // Create a Tox instance
    tox, err := toxcore.New(toxcore.NewOptions())
    if err != nil {
        log.Fatal(err)
    }
    defer tox.Kill()

    // Listen for incoming connections
    listener, err := toxnet.Listen(tox)
    if err != nil {
        log.Fatal(err)
    }
    defer listener.Close()

    fmt.Printf("Listening on: %s\\n", listener.Addr())

    // Accept connections
    conn, err := listener.Accept()
    if err != nil {
        log.Fatal(err)
    }

    // Use conn like any other net.Conn
    io.Copy(os.Stdout, conn)
}
```

## API Reference

### Functions

#### Dial
```go
func Dial(toxID string, tox *toxcore.Tox) (net.Conn, error)
```
Connects to a Tox address and returns a `net.Conn`.

#### DialTimeout
```go
func DialTimeout(toxID string, tox *toxcore.Tox, timeout time.Duration) (net.Conn, error)
```
Connects to a Tox address with a timeout.

#### DialContext
```go
func DialContext(ctx context.Context, toxID string, tox *toxcore.Tox) (net.Conn, error)
```
Connects to a Tox address with a context.

#### Listen
```go
func Listen(tox *toxcore.Tox) (net.Listener, error)
```
Creates a Tox listener that automatically accepts friend requests.

#### ListenConfig
```go
func ListenConfig(tox *toxcore.Tox, autoAccept bool) (net.Listener, error)
```
Creates a Tox listener with configuration options.

### Types

#### ToxAddr
Implements `net.Addr` for Tox addresses.

```go
type ToxAddr struct {
    // contains filtered or unexported fields
}

func NewToxAddr(toxIDString string) (*ToxAddr, error)
func NewToxAddrFromPublicKey(publicKey [32]byte, nospam [4]byte) *ToxAddr

func (a *ToxAddr) Network() string
func (a *ToxAddr) String() string
func (a *ToxAddr) PublicKey() [32]byte
func (a *ToxAddr) Nospam() [4]byte
func (a *ToxAddr) Equal(other *ToxAddr) bool
```

#### ToxConn
Implements `net.Conn` for Tox connections.

```go
type ToxConn struct {
    // contains filtered or unexported fields
}

// Standard net.Conn methods
func (c *ToxConn) Read(b []byte) (int, error)
func (c *ToxConn) Write(b []byte) (int, error)
func (c *ToxConn) Close() error
func (c *ToxConn) LocalAddr() net.Addr
func (c *ToxConn) RemoteAddr() net.Addr
func (c *ToxConn) SetDeadline(t time.Time) error
func (c *ToxConn) SetReadDeadline(t time.Time) error
func (c *ToxConn) SetWriteDeadline(t time.Time) error

// Tox-specific methods
func (c *ToxConn) FriendID() uint32
func (c *ToxConn) IsConnected() bool
```

#### ToxListener
Implements `net.Listener` for accepting Tox connections.

```go
type ToxListener struct {
    // contains filtered or unexported fields
}

// Standard net.Listener methods
func (l *ToxListener) Accept() (net.Conn, error)
func (l *ToxListener) Close() error
func (l *ToxListener) Addr() net.Addr

// Tox-specific methods
func (l *ToxListener) SetAutoAccept(autoAccept bool)
func (l *ToxListener) IsAutoAccept() bool
```

## Usage Examples

### Simple Echo Server

```go
func echoServer(tox *toxcore.Tox) {
    listener, err := toxnet.Listen(tox)
    if err != nil {
        log.Fatal(err)
    }
    defer listener.Close()

    fmt.Printf("Echo server listening on: %s\\n", listener.Addr())

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            continue
        }

        go func(c net.Conn) {
            defer c.Close()
            io.Copy(c, c) // Echo everything back
        }(conn)
    }
}
```

### Client Connection

```go
func client(targetToxID string, tox *toxcore.Tox) {
    conn, err := toxnet.DialTimeout(targetToxID, tox, 30*time.Second)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Send a message
    _, err = conn.Write([]byte("Hello from Tox!"))
    if err != nil {
        log.Fatal(err)
    }

    // Read response
    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Received: %s\\n", string(buffer[:n]))
}
```

### Working with Addresses

```go
// Parse a Tox address
addr, err := toxnet.NewToxAddr("76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37166A8712A20C018A5FA6B01")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Network: %s\\n", addr.Network())  // "tox"
fmt.Printf("Address: %s\\n", addr.String())   // Full Tox ID
fmt.Printf("Public Key: %x\\n", addr.PublicKey())
fmt.Printf("Nospam: %x\\n", addr.Nospam())

// Validate an address
if toxnet.IsToxAddr("some_address_string") {
    fmt.Println("Valid Tox address")
}

// Create from public key
publicKey := [32]byte{0x76, 0x51, 0x84, ...}
nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}
addr2 := toxnet.NewToxAddrFromPublicKey(publicKey, nospam)
```

### Connection Management

```go
// Set timeouts
conn.SetReadDeadline(time.Now().Add(30 * time.Second))
conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

// Check connection status
if toxConn, ok := conn.(*toxnet.ToxConn); ok {
    fmt.Printf("Friend ID: %d\\n", toxConn.FriendID())
    fmt.Printf("Connected: %v\\n", toxConn.IsConnected())
}
```

## Error Handling

The package provides custom error types for better error handling:

```go
import toxnet "github.com/opd-ai/toxcore/net"

conn, err := toxnet.Dial(toxID, tox)
if err != nil {
    if toxErr, ok := err.(*toxnet.ToxNetError); ok {
        fmt.Printf("Operation: %s, Error: %v\\n", toxErr.Op, toxErr.Err)
    }
    return
}
```

Common errors:
- `ErrInvalidToxID`: Invalid Tox ID format
- `ErrFriendNotFound`: Specified friend not found
- `ErrFriendOffline`: Friend is currently offline
- `ErrConnectionClosed`: Connection has been closed
- `ErrListenerClosed`: Listener has been closed
- `ErrTimeout`: Operation timed out

## Thread Safety

All types in this package are safe for concurrent use. Multiple goroutines can safely call methods on the same instances.

## Integration with Standard Library

This package is designed to work seamlessly with Go's standard networking libraries:

```go
// Works with http.Server
server := &http.Server{
    Handler: myHandler,
}
server.Serve(toxListener)

// Works with io functions
io.Copy(os.Stdout, toxConn)
io.CopyN(toxConn, strings.NewReader("data"), 4)

// Works with bufio
reader := bufio.NewReader(toxConn)
writer := bufio.NewWriter(toxConn)
```

## Requirements

- Go 1.19 or later
- A valid Tox Core instance
- Network connectivity for friend discovery and messaging

## Limitations

- Friend must be online for real-time communication
- Message size is limited by Tox protocol (chunked automatically)
- Friend requests must be accepted before communication can begin
- No built-in discovery mechanism (you need to know the target Tox ID)
