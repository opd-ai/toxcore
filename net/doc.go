// Package net provides Go standard library networking interfaces for the Tox protocol.
//
// This package implements net.Conn, net.Listener, and net.Addr interfaces to allow
// Tox-based encrypted peer-to-peer communication to work seamlessly with existing
// Go networking code.
//
// The package provides:
//   - ToxAddr: Implementation of net.Addr for Tox IDs
//   - ToxConn: Implementation of net.Conn for peer-to-peer connections
//   - ToxListener: Implementation of net.Listener for accepting connections
//   - Dial/Listen functions for establishing connections
//
// # Stream-based API (net.Conn)
//
// The stream-based API provides net.Conn semantics for reliable, ordered communication:
//
//	// Create a Tox core instance
//	tox, err := toxcore.New(toxcore.NewOptions())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Listen for incoming connections
//	listener, err := toxnet.Listen(tox)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer listener.Close()
//
//	// Accept connections
//	conn, err := listener.Accept()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use conn like any other net.Conn
//	io.Copy(os.Stdout, conn)
//
// # Packet-based API (net.PacketConn)
//
// For datagram-style communication, use the packet-based API:
//
//	// Create a packet connection (UDP-like semantics)
//	pconn, err := toxnet.PacketDial("tox", remoteToxID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pconn.Close()
//
//	// Send a datagram to a remote address
//	remoteAddr, _ := toxnet.NewToxAddr(remoteToxID)
//	n, err := pconn.WriteTo([]byte("hello"), remoteAddr)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Receive datagrams with sender address
//	buf := make([]byte, 4096)
//	n, addr, err := pconn.ReadFrom(buf)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Received %d bytes from %s\n", n, addr)
//
// For server-side packet handling with automatic connection tracking:
//
//	// Create a packet listener that wraps UDP transport
//	listener, err := toxnet.PacketListen("tox", ":8080", tox)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer listener.Close()
//
//	// Each unique remote address becomes a net.Conn via Accept()
//	for {
//	    conn, err := listener.Accept()
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    go handleConnection(conn)
//	}
//
// # Error Handling
//
// All errors are wrapped with [ToxNetError] providing context about the operation
// and address involved. Use errors.Is and errors.As for error classification:
//
//	conn, err := toxnet.Dial(toxID, tox)
//	if err != nil {
//	    var toxErr *toxnet.ToxNetError
//	    if errors.As(err, &toxErr) {
//	        log.Printf("Operation %s on %s failed: %v", toxErr.Op, toxErr.Addr, toxErr.Err)
//	    }
//	    if errors.Is(err, toxnet.ErrFriendOffline) {
//	        // Handle offline friend specifically
//	    }
//	}
//
// The implementation handles Tox-specific features like friend requests,
// message chunking/reassembly, and connection state management while
// providing familiar Go networking semantics.
package net
