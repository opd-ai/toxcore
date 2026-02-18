// Package main demonstrates the Tox async message delivery system.
//
// This example showcases three key aspects of the asynchronous messaging system:
//
//   - Demo 1: Direct Message Storage - Low-level storage operations (educational only)
//   - Demo 2: Async Manager - Forward-secure messaging with pre-key exchange
//   - Demo 3: Storage Maintenance - Message expiration and cleanup operations
//
// # Security Warning
//
// Direct storage operations (Demo 1 and Demo 3) bypass forward secrecy guarantees.
// Production applications should always use AsyncManager for secure messaging.
//
// # Usage
//
//	go run ./examples/async_demo
//
// # Dependencies
//
// This demo requires the following toxcore packages:
//   - github.com/opd-ai/toxcore/async: Async messaging system
//   - github.com/opd-ai/toxcore/crypto: Cryptographic operations
//   - github.com/opd-ai/toxcore/transport: Network transport layer
package main
