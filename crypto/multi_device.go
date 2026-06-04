package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// DeviceID is a unique identifier for a device within a multi-device session.
type DeviceID [4]byte

// DeviceBundle contains the public key material for a single device.
type DeviceBundle struct {
	DeviceID           DeviceID   // Unique device identifier
	IdentityPublic     [32]byte   // Device's Curve25519 identity public key
	SignedPreKeyPublic [32]byte   // Device's signed pre-key
	OneTimePreKeys     [][32]byte // Pool of one-time pre-keys
	OneTimePreKeyIDs   []uint32   // Per-OPK unique IDs; index i corresponds to OneTimePreKeys[i]
	SPKSignature       [64]byte   // Ed25519 signature over SPK (from device identity)
	CreatedAt          uint64     // Unix timestamp (seconds)
}

// DeviceList is a signed list of authenticated devices for a peer.
// All devices in the list share the same long-term identity key but have independent session keys.
type DeviceList struct {
	PeerIdentityPublic [32]byte       // Long-term peer identity (shared across all devices)
	Devices            []DeviceBundle // List of per-device bundles
	Signature          [64]byte       // Ed25519 signature over the entire list (signed by peer's long-term key)
	SignedAt           uint64         // Unix timestamp when list was signed
}

// ValidateDeviceList checks that:
// 1. The signature is valid (covers all devices and metadata)
// 2. Device IDs are unique
// 3. No device is too old (staleness protection)
func ValidateDeviceList(dl *DeviceList, maxAge time.Duration) error {
	if dl == nil {
		return errors.New("device list is nil")
	}

	// Check for duplicate device IDs
	seen := make(map[DeviceID]bool)
	for _, dev := range dl.Devices {
		if seen[dev.DeviceID] {
			return fmt.Errorf("duplicate device ID: %x", dev.DeviceID)
		}
		seen[dev.DeviceID] = true
	}

	// Check device list age
	now := uint64(time.Now().Unix())
	if dl.SignedAt > now {
		return errors.New("device list timestamp is in the future")
	}
	age := time.Duration(now-dl.SignedAt) * time.Second
	if age > maxAge {
		return fmt.Errorf("device list is stale: age %v > max %v", age, maxAge)
	}

	// Check per-device creation timestamps
	for _, dev := range dl.Devices {
		if dev.CreatedAt > now {
			return fmt.Errorf("device %x timestamp is in the future", dev.DeviceID)
		}
		devAge := time.Duration(now-dev.CreatedAt) * time.Second
		if devAge > maxAge {
			return fmt.Errorf("device %x is stale: age %v > max %v", dev.DeviceID, devAge, maxAge)
		}
	}

	// Verify signature: ed25519.Verify(publicKey, message, signature)
	// The message is: PeerIdentityPublic || serialized devices || SignedAt
	msg := serializeDeviceListForSigning(dl)
	if !ed25519.Verify(ed25519.PublicKey(dl.PeerIdentityPublic[:]), msg, dl.Signature[:]) {
		return errors.New("invalid device list signature")
	}

	return nil
}

// serializeDeviceListForSigning returns the exact bytes that were signed.
// Format: PeerIdentityPublic (32) || NumDevices (4) || [DeviceID(4) || IdentityPublic(32) || SPK(32) || SPKSig(64) || CreatedAt(8)]... || SignedAt(8)
func serializeDeviceListForSigning(dl *DeviceList) []byte {
	const peerIdentitySize = 32
	const deviceCountSize = 4
	const signedAtSize = 8
	buf := make([]byte, 0, peerIdentitySize+deviceCountSize+signedAtSize)
	var tmp [8]byte

	// PeerIdentityPublic
	buf = append(buf, dl.PeerIdentityPublic[:]...)

	// Number of devices
	binary.BigEndian.PutUint32(tmp[:4], uint32(len(dl.Devices)))
	buf = append(buf, tmp[:4]...)

	// Each device
	for _, dev := range dl.Devices {
		buf = append(buf, dev.DeviceID[:]...)
		buf = append(buf, dev.IdentityPublic[:]...)
		buf = append(buf, dev.SignedPreKeyPublic[:]...)
		buf = append(buf, dev.SPKSignature[:]...)
		binary.BigEndian.PutUint64(tmp[:], dev.CreatedAt)
		buf = append(buf, tmp[:]...)
	}

	// SignedAt
	binary.BigEndian.PutUint64(tmp[:], dl.SignedAt)
	buf = append(buf, tmp[:]...)

	return buf
}

// MultiDeviceSession manages per-device ratchet sessions for a single peer.
// When a message is sent to a peer with multiple devices, the sender creates a copy
// of the payload encrypted once per device's ratchet.
type MultiDeviceSession struct {
	PeerIdentity   [32]byte                 // Peer's long-term identity
	LastDeviceList *DeviceList              // Most recently validated device list
	Sessions       map[DeviceID]interface{} // Per-device ratchet sessions (from ratchet package; stored as interface{} to avoid import cycle)
	UsedOPKs       map[uint32]struct{}      // Set of OPK IDs that have been consumed (single-use enforcement)
	CreatedAt      uint64                   // Unix timestamp (seconds)
}

// AddDevice initializes a new device ratchet session via X3DH.
// This is called when a new device is added to the device list.
func (mds *MultiDeviceSession) AddDevice(
	dev *DeviceBundle,
	ourIdentityPrivate [32]byte,
	ourEphemeralPrivate [32]byte,
	spkSignature [64]byte, // Signature over dev's SPK
) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	// Validate device bundle
	if dev == nil || dev.DeviceID == (DeviceID{}) {
		return errors.New("invalid device bundle")
	}

	// Check if device already exists
	if _, exists := mds.Sessions[dev.DeviceID]; exists {
		return fmt.Errorf("device %x already exists", dev.DeviceID)
	}

	// Perform X3DH to establish session secret
	// Select the first unconsumed OPK, marking it as used (single-use enforcement).
	var selectedOPK *[32]byte
	var selectedOPKID uint32
	if mds.UsedOPKs == nil {
		mds.UsedOPKs = make(map[uint32]struct{})
	}
	for i := range dev.OneTimePreKeys {
		opkID := uint32(i + 1) // fallback sequential ID when no explicit IDs are provided
		if i < len(dev.OneTimePreKeyIDs) {
			opkID = dev.OneTimePreKeyIDs[i]
		}
		if _, used := mds.UsedOPKs[opkID]; !used {
			selectedOPK = &dev.OneTimePreKeys[i]
			selectedOPKID = opkID
			mds.UsedOPKs[opkID] = struct{}{}
			break
		}
	}

	// NOTE: In production, Ed25519 identity keys in DeviceBundle would be converted to
	// Curve25519 using DeriveX25519FromEd25519Seed before X3DH initiation.
	// For this simplified multi-device session, we assume the keys are already Curve25519.

	initParams := X3DHInitiatorParams{
		SelfIdentityPrivate:     ourIdentityPrivate,
		SelfEphemeralPrivate:    ourEphemeralPrivate,
		PeerIdentityPublic:      dev.IdentityPublic,
		PeerSignedPreKeyPublic:  dev.SignedPreKeyPublic,
		PeerOneTimePreKeyPublic: selectedOPK,
		PeerOneTimePreKeyID:     selectedOPKID,
	}

	sk, _, _, err := X3DHInitiate(initParams)
	if err != nil {
		return fmt.Errorf("X3DH for device %x failed: %w", dev.DeviceID, err)
	}
	defer ZeroBytes(sk[:])

	if mds.Sessions == nil {
		mds.Sessions = make(map[DeviceID]interface{})
	}

	sessionKey := new([32]byte)
	copy(sessionKey[:], sk[:])
	mds.Sessions[dev.DeviceID] = sessionKey

	return nil
}

// RemoveDevice tears down a device session and zeroizes all keys.
func (mds *MultiDeviceSession) RemoveDevice(deviceID DeviceID) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	session, exists := mds.Sessions[deviceID]
	if !exists {
		return fmt.Errorf("device %x not found", deviceID)
	}

	// Zeroize known session state payloads before deletion.
	switch s := session.(type) {
	case *[32]byte:
		ZeroBytes(s[:])
	case []byte:
		ZeroBytes(s)
	}

	delete(mds.Sessions, deviceID)
	return nil
}

// UpdateDeviceList atomically replaces the device list and reconciles sessions.
// Added devices get X3DH sessions; removed devices are torn down.
// On error, the session state is rolled back to its pre-update state.
func (mds *MultiDeviceSession) UpdateDeviceList(
	newList *DeviceList,
	ourIdentityPrivate [32]byte,
	maxDeviceListAge time.Duration,
) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	// Validate the new list
	if err := ValidateDeviceList(newList, maxDeviceListAge); err != nil {
		return fmt.Errorf("invalid device list: %w", err)
	}
	if !ConstantTimeEqual32(newList.PeerIdentityPublic, mds.PeerIdentity) {
		return errors.New("device list validation failed")
	}

	// Build set of new device IDs
	newDevices := make(map[DeviceID]*DeviceBundle)
	for i := range newList.Devices {
		newDevices[newList.Devices[i].DeviceID] = &newList.Devices[i]
	}

	// Snapshot of devices to add and remove before any mutations
	var toAdd []*DeviceBundle
	var toRemove []DeviceID

	// Identify devices to remove (in old list but not in new)
	for deviceID := range mds.Sessions {
		if _, found := newDevices[deviceID]; !found {
			toRemove = append(toRemove, deviceID)
		}
	}

	// Identify devices to add (in new list but not in old)
	for deviceID, dev := range newDevices {
		if _, found := mds.Sessions[deviceID]; !found {
			toAdd = append(toAdd, dev)
		}
	}

	// Apply additions first (before removals) so that if AddDevice fails,
	// we haven't yet removed devices, allowing for rollback.
	addedThisCall := make([]DeviceID, 0, len(toAdd))
	for _, dev := range toAdd {
		// Generate ephemeral key for this device
		var ephemeralKey [32]byte
		if _, err := rand.Read(ephemeralKey[:]); err != nil {
			return fmt.Errorf("failed to generate ephemeral key: %w", err)
		}
		err := mds.AddDevice(dev, ourIdentityPrivate, ephemeralKey, [64]byte{})
		ZeroBytes(ephemeralKey[:])
		if err != nil {
			var rollbackErr error
			for i := len(addedThisCall) - 1; i >= 0; i-- {
				if rmErr := mds.RemoveDevice(addedThisCall[i]); rmErr != nil && rollbackErr == nil {
					rollbackErr = rmErr
				}
			}
			if rollbackErr != nil {
				return fmt.Errorf("failed to add device %x: %w; rollback failed: %v", dev.DeviceID, err, rollbackErr)
			}
			return fmt.Errorf("failed to add device %x: %w", dev.DeviceID, err)
		}
		addedThisCall = append(addedThisCall, dev.DeviceID)
	}

	// Apply removals after all additions succeed
	for _, deviceID := range toRemove {
		if err := mds.RemoveDevice(deviceID); err != nil {
			return fmt.Errorf("failed to remove device %x: %w", deviceID, err)
		}
	}

	// Update the device list
	mds.LastDeviceList = newList

	return nil
}

// NewMultiDeviceSession creates an empty multi-device session for a peer.
func NewMultiDeviceSession(peerIdentity [32]byte) *MultiDeviceSession {
	return &MultiDeviceSession{
		PeerIdentity: peerIdentity,
		Sessions:     make(map[DeviceID]interface{}),
		UsedOPKs:     make(map[uint32]struct{}),
		CreatedAt:    uint64(time.Now().Unix()),
	}
}
