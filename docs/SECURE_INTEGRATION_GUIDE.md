# Secure Integration Guide for toxcore-go

**Date**: 2026-06-03  
**Target Audience**: Application developers integrating toxcore-go into their projects  
**Purpose**: Provide decision tables and example configurations for secure deployment

## Table of Contents

1. [Security Levels](#security-levels)
2. [Decision Table](#decision-table)
3. [Configuration Examples](#configuration-examples)
4. [Integration Patterns](#integration-patterns)
5. [Best Practices](#best-practices)
6. [Troubleshooting](#troubleshooting)

---

> **Note on API Status:** Some code examples in this guide reference `OnSecurityStatusChanged` and `SecurityStatus`, which are planned APIs that are not yet implemented. Until these are available, use `OnFriendConnectionStatus` combined with `GetFriendEncryptionStatus` to monitor security state changes. The examples have been preserved to illustrate the intended patterns.

## Security Levels

toxcore-go supports multiple security levels with automatic capability-constrained negotiation:

### Level 1: Legacy-Only (`ProtocolLegacy`)
- **Use Case**: Interoperability with classic Tox clients that only support the original protocol
- **Encryption**: NaCl-box (Curve25519 + ChaCha20-Poly1305)
- **Forward Secrecy**: Not available (per-connection only)
- **Post-Compromise Security**: Not available
- **Performance**: Minimal overhead
- **Trust Model**: TOFU (Trust-On-First-Use) with optional pinning
- **Metadata Protection**: Limited (packet sizes visible)

### Level 2: Noise-IK (`ProtocolNoiseIK`)
- **Use Case**: Default for new deployments, interop with Noise-IK capable peers
- **Encryption**: Noise-IK with Curve25519 + ChaCha20-Poly1305
- **Forward Secrecy**: Yes (DH ratcheting on new sessions)
- **Post-Compromise Security**: Partial (new-session ratchet only)
- **Performance**: ~48% overhead vs legacy (negligible for typical message rates)
- **Trust Model**: Signature-validated TOFU with automatic key-change detection
- **Metadata Protection**: Improved (cover traffic available)

### Level 3: Noise-IK + Ratchet (`ProtocolNoiseIK` with ratcheting)
- **Use Case**: Maximum security for sensitive communications
- **Encryption**: Noise-IK with per-message ratcheting + symmetric ratchet
- **Forward Secrecy**: Yes (immediate, per-message)
- **Post-Compromise Security**: Yes (Signal-like double ratchet)
- **Performance**: ~1-3% overhead vs Noise-IK (excellent security/performance ratio)
- **Trust Model**: Full signature validation with explicit key-change callbacks
- **Metadata Protection**: Maximum (cover traffic + message timing obfuscation)

### Level 4: Noise-IK + Ratchet + X3DH/PQXDH (Advanced Security)
- **Use Case**: Maximum security for sensitive communications with post-quantum protection
- **Encryption**: Noise-IK with X3DH or PQXDH initial key agreement, per-message ratcheting
- **Initial Key Agreement**: X3DH (4-DH) or PQXDH (X3DH + ML-KEM-768 hybrid)
- **Forward Secrecy**: Yes (immediate, per-message + quantum-resistant with PQXDH)
- **Post-Compromise Security**: Yes (Signal-like double ratchet)
- **Quantum Resistance**: Yes with PQXDH (harvest-now-decrypt-later protection)
- **Performance**: ~2-5% overhead vs Level 3 (excellent security/performance ratio)
- **Trust Model**: Full signature validation with explicit key-change callbacks
- **Metadata Protection**: Maximum (sealed sender + cover traffic + message timing obfuscation)
- **Additional Features**:
  - Sealed sender (encrypts sender identity to prevent transport-layer identification)
  - Double Ratchet header encryption (hides sequence numbers and ratchet state)

All security features are enabled via capability negotiation (`CapX3DH`, `CapPQXDH`,
`CapHeaderEncryption`) and automatically downgrade when communicating with peers that
don't support them, maintaining full backward compatibility.

---

## Decision Table

Use this table to determine which security level to use for your use case:

| Use Case | Requirement | Legacy-Only | Noise-IK | Noise-IK+Ratchet | Noise-IK+Ratchet+PQXDH | Recommendation |
|----------|-------------|:---:|:---:|:---:|:---:|---|
| **Legacy Interop** | Must talk to old Tox clients | ✅ | ⚠️* | ⚠️* | ⚠️* | **Legacy-Only** |
| **Desktop App** | Modern peer, max security | ❌ | ✅ | ✅✅ | ✅✅✅ | **Noise-IK+Ratchet+PQXDH** |
| **Mobile App** | Balance security & battery | ❌ | ✅✅ | ✅✅ | ✅ | **Noise-IK+Ratchet** |
| **Public Chat** | Group messaging, many peers | ⚠️ | ✅✅ | ✅ | ⚠️ | **Noise-IK** |
| **Sensitive Comms** | Medical/legal/financial | ❌ | ⚠️ | ✅✅ | ✅✅✅ | **Noise-IK+Ratchet+PQXDH** |
| **IoT Device** | Resource-constrained | ✅✅ | ⚠️ | ❌ | ❌ | **Legacy-Only** |
| **Browser Client** | Web-based Tox | ❌ | ✅✅ | ✅ | ⚠️ | **Noise-IK** |
| **Backup/Archive** | Long-term message store | ❌ | ✅✅ | ✅✅ | ✅✅✅ | **Noise-IK+Ratchet+PQXDH** |
| **Quantum-Resistant** | Post-quantum security required | ❌ | ❌ | ⚠️ | ✅✅✅ | **Noise-IK+Ratchet+PQXDH** |

**Legend**:
- ✅ Recommended
- ✅✅ Highly recommended
- ✅✅✅ Maximum security
- ⚠️ Fallback behavior (automatic downgrade if peer doesn't support)
- ❌ Not recommended

*Noise-IK, Ratchet, and PQXDH automatically fall back to lower levels if peer doesn't support them.

---

## Configuration Examples

### Example 1: Secure Desktop Application (Noise-IK + Ratchet)

```go
package main

import (
	"log"
	"github.com/opd-ai/toxcore"
)

func main() {
	// Create options with security defaults
	options := toxcore.NewOptions()
	
	// These settings are automatically secure by default:
	// - Noise-IK is enabled
	// - Ratcheting enabled if peer supports
	// - Encrypted negotiation
	// - Signature validation required
	options.UDPEnabled = true
	options.TCPPort = 0  // Auto-select, for relay/fallback
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal("Failed to create Tox instance:", err)
	}
	defer tox.Kill()
	
	// Register connection/security callback using existing toxcore APIs
	tox.OnFriendConnectionStatus(func(friendID uint32, connectionStatus toxcore.ConnectionStatus) {
		encryption := tox.GetFriendEncryptionStatus(friendID)
		log.Printf("Friend %d connection: %v, encryption: %s", friendID, connectionStatus, encryption)
		if encryption == toxcore.EncryptionLegacy {
			log.Printf("⚠️  SECURITY WARNING: Friend %d is using legacy encryption", friendID)
		}
	})
	
	// All messages are automatically encrypted with the highest mutually supported level
	friendID := uint32(1)
	err = tox.SendFriendMessage(friendID, "Securely encrypted message")
	if err != nil {
		log.Printf("Failed to send message (encryption required): %v", err)
		// This error means encryption prerequisites are not met
		// Application should ensure proper key exchange before sending
	}
}
```

### Example 2: Mobile App (Noise-IK, Battery-Conscious)

```go
package main

import (
	"log"
	"github.com/opd-ai/toxcore"
)

func main() {
	options := toxcore.NewOptions()
	
	// Mobile-specific settings
	options.UDPEnabled = true
	options.TCPPort = 0
	
	// Disable privacy features that consume battery on mobile
	// (but keep default Noise-IK encryption)
	// Note: Set via environment or separate config if needed
	// For now, use default Noise-IK without heavy cover traffic
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()
	
	// Monitor connection status changes (encryption level can be queried via GetFriendEncryptionStatus)
	// NOTE: OnSecurityStatusChanged is planned but not yet implemented.
	// Use OnFriendConnectionStatus to detect when friends come online/offline.
	tox.OnFriendConnectionStatus(func(friendID uint32, status toxcore.ConnectionStatus) {
		if status != toxcore.ConnectionNone {
			// Friend connected - check encryption level
			encStatus := tox.GetFriendEncryptionStatus(friendID)
			if encStatus == toxcore.EncryptionLegacy {
				log.Printf("⚠️ Friend %d using legacy encryption", friendID)
			}
		}
	})
	
	// Send message with automatic encryption
	tox.SendFriendMessage(1, "Mobile app message (Noise-IK encrypted)")
}
```

### Example 3: IoT Device (Legacy-Only, Minimal Resource)

```go
package main

import (
	"log"
	"github.com/opd-ai/toxcore"
)

func main() {
	options := toxcore.NewOptions()
	
	// IoT-specific: Legacy-only for minimal overhead
	options.UDPEnabled = true
	// Disable Noise-IK if possible (implementation-dependent)
	// For now, Noise-IK is default but will auto-fall-back to Legacy-Only
	// if peer doesn't support it
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()
	
	// Minimal event handling
	tox.OnFriendMessage(func(friendID uint32, message string) {
		log.Printf("Received from friend %d: %s", friendID, message)
	})
	
	// Send encrypted message (will use best available security)
	tox.SendFriendMessage(1, "IoT sensor data")
}
```

### Example 4: Backend Service (Noise-IK + Ratchet, with Verification)

```go
package main

import (
	"log"
	"github.com/opd-ai/toxcore"
	"github.com/sirupsen/logrus"
)

func main() {
	// Set up structured logging for security events
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	
	options := toxcore.NewOptions()
	options.UDPEnabled = true
	options.TCPPort = 33445 // Standard Tox port
	
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatal(err)
	}
	defer tox.Kill()
	
	// Store peer fingerprints for out-of-band verification
	var trustedPeers = make(map[uint32][32]byte)
	
	// Verify all new friends out-of-band before trusting
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		logger.WithFields(logrus.Fields{
			"public_key": publicKey,
			"message":    message,
		}).Info("Received friend request - VERIFY OUT-OF-BAND BEFORE ACCEPTING")
		
		// In production, verify the public key matches your records
		// using a separate channel (QR code, phone call, etc.)
		friendID, err := tox.AddFriendByPublicKey(publicKey)
		if err != nil {
			logger.WithError(err).Error("Failed to add friend")
			return
		}
		trustedPeers[friendID] = publicKey
	})
	
	// Monitor ratcheting status
	tox.OnSecurityStatusChanged(func(friendID uint32, status toxcore.SecurityStatus) {
		logger.WithFields(logrus.Fields{
			"friend_id":          friendID,
			"current_protocol":   status.CurrentProtocol,
			"ratchet_available":  status.RatchetAvailable,
			"ratchet_enabled":    status.RatchetEnabled,
			"downgraded":         status.Downgraded,
			"skipped_keys_count": status.SkippedKeysCount,
		}).Info("Security status update")
	})
	
	// Handle messages with full security context
	tox.OnFriendMessage(func(friendID uint32, message string) {
		// Application layer should check SecurityStatus
		// before processing sensitive operations
		logger.WithFields(logrus.Fields{
			"friend_id": friendID,
		}).Debug("Message received (auto-encrypted by toxcore)")
	})
}
```

---

## Integration Patterns

### Pattern 1: Simple Chat Application

```go
func setupSecureChat() {
	options := toxcore.NewOptions()
	tox, _ := toxcore.New(options)
	
	// 1. Set up callbacks FIRST
	tox.OnFriendRequest(handleFriendRequest)
	tox.OnFriendMessage(handleMessage)
	tox.OnSecurityStatusChanged(handleSecurityUpdate)
	
	// 2. Start the event loop
	go func() {
		for {
			tox.Iterate()
			time.Sleep(50 * time.Millisecond)
		}
	}()
	
	// 3. Bootstrap into the DHT
	tox.Bootstrap(
		"104.200.140.11",    // Public node
		33445,
		toxCorePublicKey,
	)
	
	// 4. Exchange Tox IDs out-of-band
	// (This is crucial for verification)
	myID := tox.SelfGetAddress()
	log.Println("My Tox ID:", myID)
	
	// 5. When friend comes online, messages are automatically encrypted
	tox.SendFriendMessage(friendID, "Hello!")
}
```

### Pattern 2: Daemon/Service with File Persistence

```go
func setupSecureService() {
	const saveFile = "/var/lib/myapp/tox_profile.dat"
	
	// 1. Load existing profile or create new
	var data []byte
	if _, err := os.Stat(saveFile); err == nil {
		data, _ = ioutil.ReadFile(saveFile)
	}
	
	options := toxcore.NewOptions()
	options.Savedata = data
	tox, _ := toxcore.New(options)
	
	// 2. Save profile periodically
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			data := tox.GetSavedata()
			ioutil.WriteFile(saveFile, data, 0600)
		}
	}()
	
	// 3. Register security callbacks to log to syslog
	tox.OnSecurityStatusChanged(func(friendID uint32, status toxcore.SecurityStatus) {
		logToSyslog(fmt.Sprintf("Security update: friend=%d protocol=%s", 
			friendID, status.CurrentProtocol))
	})
}
```

### Pattern 3: Strict Verification (Out-of-Band Key Exchange)

```go
func setupVerifiedPeers() {
	tox, _ := toxcore.New(toxcore.NewOptions())
	
	// Hardcode trusted peer IDs (obtain via QR code, phone call, etc.)
	var trustedPublicKeys = map[string][32]byte{
		"alice": parseToxPublicKey("ALICE_PUBLIC_KEY_HEX"),
		"bob":   parseToxPublicKey("BOB_PUBLIC_KEY_HEX"),
	}
	
	// On friend request, verify public key matches our records
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		allowed := false
		for name, trustedKey := range trustedPublicKeys {
			if publicKey == trustedKey {
				allowed = true
				log.Printf("Auto-accepting friend request from %s", name)
				break
			}
		}
		
		if !allowed {
			log.Printf("REJECTING friend request from unknown peer: %x", publicKey)
			return
		}
		
		// Accept only verified peers
		tox.AddFriendByPublicKey(publicKey)
	})
}
```

---

## Best Practices

### 1. Always Verify Keys Out-of-Band

```go
// ❌ DON'T: Accept friend requests without verification
tox.OnFriendRequest(func(publicKey [32]byte, message string) {
	friendID, _ := tox.AddFriendByPublicKey(publicKey)
	log.Println("Added friend:", friendID)
})

// ✅ DO: Verify the key through a separate trusted channel
tox.OnFriendRequest(func(publicKey [32]byte, message string) {
	// Display this to the user via QR code, show fingerprint, etc.
	fingerprint := computeFingerprint(publicKey)
	log.Printf("Friend sent: %s, Fingerprint: %s - Verify with them!", 
		message, fingerprint)
	
	// Only accept after user confirms
	if userVerifiedFingerprint(fingerprint) {
		tox.AddFriendByPublicKey(publicKey)
	}
})
```

### 2. Monitor Security Status Changes

```go
// ✅ DO: Log and alert on security status changes
tox.OnSecurityStatusChanged(func(friendID uint32, status toxcore.SecurityStatus) {
	if status.Downgraded {
		// CRITICAL: Connection downgraded
		log.Printf("🚨 SECURITY ALERT: Friend %d connection downgraded to %s", 
			friendID, status.CurrentProtocol)
		// Notify user, pause sensitive operations
	}
	
	if status.SkippedKeysCount > 10 {
		// WARNING: Many skipped keys (possible MitM attempt)
		log.Printf("⚠️  Suspicious activity: %d skipped keys for friend %d", 
			status.SkippedKeysCount, friendID)
	}
})
```

### 3. Never Log Plaintext Messages

```go
// ❌ DON'T: Log message content
tox.OnFriendMessage(func(friendID uint32, message string) {
	log.Printf("Received from %d: %s", friendID, message)  // INSECURE!
})

// ✅ DO: Log metadata only, not content
tox.OnFriendMessage(func(friendID uint32, message string) {
	log.Printf("Received from friend %d, length=%d", friendID, len(message))
})
```

### 4. Implement Rate Limiting

```go
// ✅ DO: Implement rate limiting for incoming messages
var messageRates = make(map[uint32]*RateLimiter)

tox.OnFriendMessage(func(friendID uint32, message string) {
	if _, exists := messageRates[friendID]; !exists {
		messageRates[friendID] = NewRateLimiter(10, time.Second)
	}
	
	if !messageRates[friendID].Allow() {
		log.Printf("Rate limit exceeded for friend %d", friendID)
		return
	}
	
	// Process message
	processMessage(friendID, message)
})
```

### 5. Save Profile Securely

```go
// ✅ DO: Save with restricted permissions
data := tox.GetSavedata()
err := ioutil.WriteFile(profilePath, data, 0600)  // Owner read/write only
if err != nil {
	log.Fatal(err)
}

// ✅ DO: Encrypt profile on disk if possible
encryptedData := encryptProfileData(data, masterKey)
ioutil.WriteFile(profilePath+".enc", encryptedData, 0600)
```

### 6. Handle Errors Properly

```go
// ❌ DON'T: Ignore encryption errors
tox.SendFriendMessage(friendID, sensitiveData)

// ✅ DO: Check for encryption errors
err := tox.SendFriendMessage(friendID, sensitiveData)
if err != nil {
	log.Printf("Failed to send encrypted message: %v", err)
	// The message was NOT sent
	// Application must decide how to handle this:
	// - Retry after key exchange
	// - Notify user
	// - Queue for later
}
```

---

## Troubleshooting

### Issue: "Encryption not available" errors

**Cause**: Peer key exchange hasn't completed yet.

**Solution**:
```go
// Wait for friend to come online
tox.OnFriendConnectionStatusChanged(func(friendID uint32, status toxcore.ConnectionStatus) {
	if status.IsConnected() {
		log.Printf("Friend %d now online", friendID)
		// Safe to send encrypted messages now
	}
})
```

### Issue: Connection downgraded to Legacy

**Cause**: Peer doesn't support Noise-IK.

**Solution**: This is expected for interoperability. Legacy-Only is still encrypted:
```go
tox.OnSecurityStatusChanged(func(friendID uint32, status toxcore.SecurityStatus) {
	if status.CurrentProtocol == "legacy" {
		log.Printf("Friend %d using legacy protocol (still encrypted)", friendID)
		// This is acceptable - message is still encrypted with NaCl-box
	}
})
```

### Issue: Skipped keys detected

**Cause**: Normal behavior during ratcheting. Indicates replay protection is working.

**Solution**: Monitor but don't panic:
```go
if status.SkippedKeysCount > 100 {
	log.Printf("⚠️ WARNING: Unusual number of skipped keys: %d", 
		status.SkippedKeysCount)
	// This could indicate message flooding or MitM attack
	// Consider rate limiting or disconnecting
}
```

### Issue: Performance degradation with many friends

**Cause**: Security callbacks and logging overhead.

**Solution**:
```go
// Use conditional logging
if log.Level >= logrus.DebugLevel {
	tox.OnSecurityStatusChanged(handleSecurityUpdate)
}

// Or use sampling
if friendID % 10 == 0 {  // Log every 10th friend
	log.Printf("Status update for friend %d", friendID)
}
```

---

## Summary

| Security Level | Desktop | Mobile | IoT | Backend |
|---|:---:|:---:|:---:|:---:|
| **Legacy-Only** | ❌ | ❌ | ✅ | ❌ |
| **Noise-IK** | ✅ | ✅✅ | ✅* | ✅ |
| **Noise-IK+Ratchet** | ✅✅ | ✅ | ❌ | ✅✅ |

**Key Takeaways**:
1. All connections are **encrypted by default**
2. Downgrade is **automatic and capability-constrained** (no user toggle)
3. **Verify keys out-of-band** before trusting new peers
4. **Monitor security status** for unexpected downgrades
5. **Never log message content**, only metadata
6. Use **Noise-IK+Ratchet for sensitive communications**
7. **Handle encryption errors** - they mean prerequisites are unmet

For additional security guidance, see:
- [SECURITY.md](../SECURITY.md) - Overall security model
- [PROTOCOL_SPEC.md](./PROTOCOL_SPEC.md) - Protocol details
- [PROFILE_GUIDED_OPTIMIZATION.md](./PROFILE_GUIDED_OPTIMIZATION.md) - Performance guidelines
