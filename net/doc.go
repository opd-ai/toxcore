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
// Example usage:
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
// The implementation handles Tox-specific features like friend requests,
// message chunking/reassembly, and connection state management while
// providing familiar Go networking semantics.
package net
