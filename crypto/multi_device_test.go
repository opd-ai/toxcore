package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMultiDeviceSessionCreation verifies that a multi-device session can be created.
func TestMultiDeviceSessionCreation(t *testing.T) {
	t.Parallel()

	var peerIdentity [32]byte
	rand.Read(peerIdentity[:])

	mds := NewMultiDeviceSession(peerIdentity)
	require.NotNil(t, mds)
	require.Equal(t, mds.PeerIdentity, peerIdentity)
	require.NotNil(t, mds.Sessions)
	require.Empty(t, mds.Sessions)
	require.True(t, mds.CreatedAt > 0)
}

// TestDeviceListValidation tests validation of device lists.
func TestDeviceListValidation(t *testing.T) {
	t.Parallel()

	// Generate valid keys
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var peerPub [32]byte
	copy(peerPub[:], pubKey)

	// Create a valid device
	var devID DeviceID
	devID[0] = 1
	var devIdentity, spkPub [32]byte
	rand.Read(devIdentity[:])
	rand.Read(spkPub[:])

	dev := DeviceBundle{
		DeviceID:           devID,
		IdentityPublic:     devIdentity,
		SignedPreKeyPublic: spkPub,
		CreatedAt:          uint64(time.Now().Unix()),
	}

	// Create device list
	now := uint64(time.Now().Unix())
	dl := &DeviceList{
		PeerIdentityPublic: peerPub,
		Devices:            []DeviceBundle{dev},
		SignedAt:           now,
	}

	// Sign the list
	msg := serializeDeviceListForSigning(dl)
	sig := ed25519.Sign(privKey, msg)
	copy(dl.Signature[:], sig)

	// Validate
	err := ValidateDeviceList(dl, 1*time.Hour)
	require.NoError(t, err)
}

// TestDeviceListDuplicateDetection verifies that duplicate device IDs are rejected.
func TestDeviceListDuplicateDetection(t *testing.T) {
	t.Parallel()

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var peerPub [32]byte
	copy(peerPub[:], pubKey)

	// Create two devices with the same ID
	var devID DeviceID
	devID[0] = 1

	dev1 := DeviceBundle{
		DeviceID:  devID,
		CreatedAt: uint64(time.Now().Unix()),
	}

	dev2 := DeviceBundle{
		DeviceID:  devID, // Duplicate!
		CreatedAt: uint64(time.Now().Unix()),
	}

	now := uint64(time.Now().Unix())
	dl := &DeviceList{
		PeerIdentityPublic: peerPub,
		Devices:            []DeviceBundle{dev1, dev2},
		SignedAt:           now,
	}

	// Validation should fail due to duplicate device ID
	err := ValidateDeviceList(dl, 1*time.Hour)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate device ID")
}

// TestDeviceListStalenessDetection verifies that old device lists are rejected.
func TestDeviceListStalenessDetection(t *testing.T) {
	t.Parallel()

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var peerPub [32]byte
	copy(peerPub[:], pubKey)

	var devID DeviceID
	devID[0] = 1
	dev := DeviceBundle{
		DeviceID:  devID,
		CreatedAt: uint64(time.Now().Unix()),
	}

	// Create an old device list (signed 2 hours ago, but max age is 1 hour)
	oldTime := uint64(time.Now().Add(-2 * time.Hour).Unix())
	dl := &DeviceList{
		PeerIdentityPublic: peerPub,
		Devices:            []DeviceBundle{dev},
		SignedAt:           oldTime,
	}

	msg := serializeDeviceListForSigning(dl)
	// Use ed25519.Sign directly to avoid pointer issues
	sig := ed25519.Sign(privKey, msg)
	copy(dl.Signature[:], sig)

	// Validation should fail due to staleness
	err := ValidateDeviceList(dl, 1*time.Hour)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stale")
}

// TestDeviceRemoval tests removing a device from a multi-device session.
func TestDeviceRemoval(t *testing.T) {
	t.Parallel()

	var peerIdentity [32]byte
	rand.Read(peerIdentity[:])

	mds := NewMultiDeviceSession(peerIdentity)

	var devID DeviceID
	devID[0] = 1

	// Manually add to sessions map to test removal
	mds.Sessions[devID] = nil

	require.Len(t, mds.Sessions, 1)

	// Remove device
	err := mds.RemoveDevice(devID)
	require.NoError(t, err)
	require.Len(t, mds.Sessions, 0)
}

// TestMultiDeviceKeyLifecycle verifies that sessions are properly managed.
func TestMultiDeviceKeyLifecycle(t *testing.T) {
	t.Parallel()

	_, peerPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	peerPub := peerPriv.Public().(ed25519.PublicKey)

	var peerIdentity [32]byte
	copy(peerIdentity[:], peerPub)

	mds := NewMultiDeviceSession(peerIdentity)

	var ourIdentityPrivate [32]byte
	rand.Read(ourIdentityPrivate[:])

	makeDevice := func(id uint32) DeviceBundle {
		var devID DeviceID
		binary.BigEndian.PutUint32(devID[:], id)

		var idPriv, spkPriv [32]byte
		rand.Read(idPriv[:])
		rand.Read(spkPriv[:])
		idPub, _ := deriveX25519Public(idPriv)
		spkPub, _ := deriveX25519Public(spkPriv)

		return DeviceBundle{
			DeviceID:           devID,
			IdentityPublic:     idPub,
			SignedPreKeyPublic: spkPub,
			CreatedAt:          uint64(time.Now().Unix()),
		}
	}

	signList := func(dl *DeviceList) {
		msg := serializeDeviceListForSigning(dl)
		sig := ed25519.Sign(peerPriv, msg)
		copy(dl.Signature[:], sig)
	}

	dev1 := makeDevice(1)
	dev2 := makeDevice(2)
	dev3 := makeDevice(3)

	now := uint64(time.Now().Unix())
	list1 := &DeviceList{
		PeerIdentityPublic: peerIdentity,
		Devices:            []DeviceBundle{dev1, dev2, dev3},
		SignedAt:           now,
	}
	signList(list1)

	err = mds.UpdateDeviceList(list1, ourIdentityPrivate, 1*time.Hour)
	require.NoError(t, err)
	require.Len(t, mds.Sessions, 3)

	list2 := &DeviceList{
		PeerIdentityPublic: peerIdentity,
		Devices:            []DeviceBundle{dev1, dev3},
		SignedAt:           uint64(time.Now().Unix()),
	}
	signList(list2)

	err = mds.UpdateDeviceList(list2, ourIdentityPrivate, 1*time.Hour)
	require.NoError(t, err)
	require.Len(t, mds.Sessions, 2)

	_, exists := mds.Sessions[dev2.DeviceID]
	require.False(t, exists)
}

// TestDeviceListSignatureSerialization verifies that device list serialization is consistent.
func TestDeviceListSignatureSerialization(t *testing.T) {
	t.Parallel()

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := privKey.Public().(ed25519.PublicKey)

	var peerPub [32]byte
	copy(peerPub[:], pubKey)

	// Create a device list
	var devID DeviceID
	devID[0] = 1
	var devIdentity, spkPub [32]byte
	rand.Read(devIdentity[:])
	rand.Read(spkPub[:])

	dev := DeviceBundle{
		DeviceID:           devID,
		IdentityPublic:     devIdentity,
		SignedPreKeyPublic: spkPub,
		CreatedAt:          uint64(time.Now().Unix()),
	}

	now := uint64(time.Now().Unix())
	dl := &DeviceList{
		PeerIdentityPublic: peerPub,
		Devices:            []DeviceBundle{dev},
		SignedAt:           now,
	}

	// Serialize twice and verify it's deterministic
	msg1 := serializeDeviceListForSigning(dl)
	msg2 := serializeDeviceListForSigning(dl)

	require.Equal(t, msg1, msg2, "device list serialization must be deterministic")
}
