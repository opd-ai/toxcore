// Package main demonstrates the bootstrap server API from the bootstrap package.
//
// It shows how to run a Tox DHT bootstrap node on clearnet (UDP), Tor onion
// services, and I2P — all sharing the same public key.
//
// Usage:
//
//	go run . [--onion] [--i2p] [--port 33445]
//
// Flags:
//
//	--onion     Enable Tor hidden-service endpoint (requires running Tor daemon)
//	--i2p       Enable I2P endpoint (requires running I2P router with SAM bridge)
//	--port N    UDP port to bind for clearnet (default 33445)
package main
