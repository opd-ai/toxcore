// Package main provides a demonstration of the async messaging system with
// automatic identity obfuscation.
//
// This example showcases the completed Week 2 integration of the toxcore async
// messaging system, where all APIs now use obfuscation by default:
//
//   - AsyncClient.SendAsyncMessage() automatically uses obfuscation
//   - AsyncClient.RetrieveAsyncMessages() uses pseudonym-based retrieval
//   - AsyncManager provides storage node integration with forward secrecy
//
// # Usage
//
// Run the demo from the command line:
//
//	go run .
//
// The demo creates two users (Alice and Bob) and demonstrates:
//   - Legacy API compatibility with automatic obfuscation
//   - Input validation with obfuscated messaging
//   - Storage node operation with pseudonym-based identity protection
//   - Manager integration with forward secrecy
//
// # Privacy Protection
//
// The obfuscation layer provides:
//   - Peer identity protection from storage nodes
//   - Cryptographic pseudonyms instead of real public keys
//   - Forward secrecy for all messages
//   - End-to-end encryption maintained throughout
//
// # Requirements
//
// The demo requires UDP ports 8001-8004 to be available for local transport creation.
package main
