// Package bootstrap provides a simple, straightforward API for running a Tox
// DHT bootstrap server on clearnet (UDP), Tor onion services, and I2P.
//
// A bootstrap server is a well-known Tox DHT node that new peers connect to in
// order to discover other nodes in the network. This package makes it easy to
// run such a server on multiple overlay networks simultaneously.
//
// # Quick Start
//
// Clearnet-only bootstrap server:
//
//	cfg := bootstrap.DefaultConfig()
//	cfg.ClearnetPort = 33445
//
//	srv, err := bootstrap.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := srv.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer srv.Stop()
//
//	fmt.Printf("Bootstrap server running\n")
//	fmt.Printf("  Clearnet: %s\n", srv.GetClearnetAddr())
//	fmt.Printf("  Public key: %s\n", srv.GetPublicKeyHex())
//
// Multi-network bootstrap server (clearnet + Tor + I2P):
//
//	cfg := bootstrap.DefaultConfig()
//	cfg.ClearnetPort = 33445
//	cfg.OnionEnabled = true    // requires Tor daemon with control port
//	cfg.I2PEnabled  = true    // requires I2P router with SAM bridge
//
//	srv, err := bootstrap.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := srv.Start(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//	defer srv.Stop()
//
//	fmt.Printf("Clearnet addr: %s\n", srv.GetClearnetAddr())
//	fmt.Printf("Onion addr:    %s\n", srv.GetOnionAddr())
//	fmt.Printf("I2P addr:      %s\n", srv.GetI2PAddr())
//	fmt.Printf("Public key:    %s\n", srv.GetPublicKeyHex())
//
// # Network Types
//
// Clearnet uses standard UDP on a public IP address and port. This is the
// traditional Tox bootstrap mode and is always available.
//
// Onion mode runs the bootstrap server as a Tor v3 hidden service (.onion).
// It requires a running Tor daemon with the control port enabled (default
// 127.0.0.1:9051, configurable via the TOR_CONTROL_ADDR environment variable).
// Key persistence is handled automatically by the onramp library in the
// onionkeys/ directory.
//
// I2P mode runs the bootstrap server as an I2P destination (.b32.i2p).
// It requires a running I2P router (i2pd or Java I2P) with the SAM bridge
// enabled (default 127.0.0.1:7656, configurable via the I2P_SAM_ADDR
// environment variable). Key persistence is handled automatically by the
// onramp library in the i2pkeys/ directory.
//
// # Shared Identity
//
// All three network endpoints share the same Tox public key. Clients can
// verify they are connecting to the same bootstrap node regardless of which
// network they use.
//
// # Advertising the Server
//
// After calling Start, retrieve the addresses and public key to publish them
// in bootstrap node lists:
//
//	fmt.Printf("address = %s\nport = %d\npublic_key = %s\n",
//	    srv.GetClearnetAddr(), cfg.ClearnetPort, srv.GetPublicKeyHex())
package bootstrap
