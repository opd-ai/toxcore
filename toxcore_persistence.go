// toxcore_persistence.go contains serialization and persistence functionality for the Tox instance.
// This file is part of the toxcore package refactoring to improve maintainability.
package toxcore

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// marshal serializes the toxSaveData to a JSON byte array.
// Using JSON for simplicity and readability during development.
// Future versions could use a binary format for efficiency.
func (s *toxSaveData) marshal() []byte {
	// Import encoding/json at the top of file
	data, err := json.Marshal(s)
	if err != nil {
		// In case of marshaling error, return empty data
		// This prevents panic while allowing graceful degradation
		return []byte{}
	}
	return data
}

// unmarshal deserializes JSON data into toxSaveData.
func (s *toxSaveData) unmarshal(data []byte) error {
	return json.Unmarshal(data, s)
}

// marshalBinary serializes the toxSaveData to a binary format for faster recovery.
// Format: [4B magic][2B version][2B flags][8B timestamp][32B pubkey][32B secretkey]
//
//	[4B nospam][2B name_len][name][2B status_len][status][4B friends_count][friends...]
func (s *toxSaveData) marshalBinary() ([]byte, error) {
	// Calculate size (approximate, will grow buffer if needed)
	estimatedSize := 4 + 2 + 2 + 8 + 32 + 32 + 4 + 2 + len(s.SelfName) + 2 + len(s.SelfStatusMsg) + 4
	for _, f := range s.Friends {
		estimatedSize += 32 + 1 + 1 + 2 + len(f.Name) + 2 + len(f.StatusMessage) + 8 + 4
	}
	buf := make([]byte, 0, estimatedSize)

	// Header
	buf = binary.BigEndian.AppendUint32(buf, SnapshotMagic)
	buf = binary.BigEndian.AppendUint16(buf, SnapshotVersion)
	buf = binary.BigEndian.AppendUint16(buf, 0) // flags (reserved)
	buf = binary.BigEndian.AppendUint64(buf, uint64(time.Now().UnixNano()))

	// KeyPair
	if s.KeyPair != nil {
		buf = append(buf, s.KeyPair.Public[:]...)
		buf = append(buf, s.KeyPair.Private[:]...)
	} else {
		buf = append(buf, make([]byte, 64)...)
	}

	// Nospam
	buf = append(buf, s.Nospam[:]...)

	// Self info
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(s.SelfName)))
	buf = append(buf, []byte(s.SelfName)...)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(s.SelfStatusMsg)))
	buf = append(buf, []byte(s.SelfStatusMsg)...)

	// Friends
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(s.Friends)))
	for friendID, f := range s.Friends {
		buf = binary.BigEndian.AppendUint32(buf, friendID)
		buf = append(buf, f.PublicKey[:]...)
		buf = append(buf, byte(f.Status))
		buf = append(buf, byte(f.ConnectionStatus))
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(f.Name)))
		buf = append(buf, []byte(f.Name)...)
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(f.StatusMessage)))
		buf = append(buf, []byte(f.StatusMessage)...)
		buf = binary.BigEndian.AppendUint64(buf, uint64(f.LastSeen.UnixNano()))
	}

	return buf, nil
}

// snapshotReader wraps binary data and an offset for sequential reading.
type snapshotReader struct {
	data   []byte
	offset int
}

// remaining returns the number of unread bytes.
func (r *snapshotReader) remaining() int {
	return len(r.data) - r.offset
}

// ensureBytes checks that at least n bytes remain in the data.
func (r *snapshotReader) ensureBytes(n int, context string) error {
	if len(r.data) < r.offset+n {
		return fmt.Errorf("snapshot truncated at %s", context)
	}
	return nil
}

// readUint16 reads a big-endian uint16 and advances the offset.
func (r *snapshotReader) readUint16(context string) (uint16, error) {
	if err := r.ensureBytes(2, context); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint16(r.data[r.offset:])
	r.offset += 2
	return v, nil
}

// readUint32 reads a big-endian uint32 and advances the offset.
func (r *snapshotReader) readUint32(context string) (uint32, error) {
	if err := r.ensureBytes(4, context); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint32(r.data[r.offset:])
	r.offset += 4
	return v, nil
}

// readBytes reads exactly n bytes and advances the offset.
func (r *snapshotReader) readBytes(n int, context string) ([]byte, error) {
	if err := r.ensureBytes(n, context); err != nil {
		return nil, err
	}
	b := r.data[r.offset : r.offset+n]
	r.offset += n
	return b, nil
}

// readLengthPrefixedString reads a uint16 length followed by that many bytes as a string.
func (r *snapshotReader) readLengthPrefixedString(context string) (string, error) {
	length, err := r.readUint16(context + " length")
	if err != nil {
		return "", err
	}
	b, err := r.readBytes(int(length), context)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// skip advances the offset by n bytes without returning data.
func (r *snapshotReader) skip(n int) {
	r.offset += n
}

// unmarshalBinary deserializes binary snapshot data into toxSaveData.
func (s *toxSaveData) unmarshalBinary(data []byte) error {
	if len(data) < 86 { // Minimum header size
		return errors.New("snapshot data too short")
	}

	r := &snapshotReader{data: data}

	if err := s.unmarshalHeader(r); err != nil {
		return err
	}
	if err := s.unmarshalKeyPair(r); err != nil {
		return err
	}
	if err := s.unmarshalSelfInfo(r); err != nil {
		return err
	}
	return s.unmarshalFriends(r)
}

// unmarshalHeader validates the snapshot magic, version, and skips flags/timestamp.
func (s *toxSaveData) unmarshalHeader(r *snapshotReader) error {
	magic, err := r.readUint32("magic")
	if err != nil {
		return err
	}
	if magic != SnapshotMagic {
		return errors.New("invalid snapshot magic")
	}

	version, err := r.readUint16("version")
	if err != nil {
		return err
	}
	if version > SnapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d", version)
	}

	if _, err := r.readBytes(2, "flags"); err != nil {
		return err
	}
	if _, err := r.readBytes(8, "timestamp"); err != nil {
		return err
	}
	return nil
}

// unmarshalKeyPair reads the public and private keys from the snapshot.
func (s *toxSaveData) unmarshalKeyPair(r *snapshotReader) error {
	keyData, err := r.readBytes(64, "keypair")
	if err != nil {
		return err
	}
	var pubKey, secKey [32]byte
	copy(pubKey[:], keyData[:32])
	copy(secKey[:], keyData[32:64])

	var zeroKey [32]byte
	if pubKey != zeroKey {
		s.KeyPair = &crypto.KeyPair{
			Public:  pubKey,
			Private: secKey,
		}
	}
	return nil
}

// unmarshalSelfInfo reads nospam, self name, and status message.
func (s *toxSaveData) unmarshalSelfInfo(r *snapshotReader) error {
	nospamData, err := r.readBytes(4, "nospam")
	if err != nil {
		return err
	}
	copy(s.Nospam[:], nospamData)

	s.SelfName, err = r.readLengthPrefixedString("self name")
	if err != nil {
		return err
	}
	s.SelfStatusMsg, err = r.readLengthPrefixedString("status message")
	return err
}

// unmarshalFriends reads the friends list from the snapshot.
func (s *toxSaveData) unmarshalFriends(r *snapshotReader) error {
	friendsCount, err := r.readUint32("friends count")
	if err != nil {
		return err
	}

	// Minimum bytes per friend entry: 4 (ID) + 32 (PK) + 2 (status) + 2 (name len) + 2 (status len) + 8 (last seen) = 50
	const minFriendEntrySize = 50
	maxPossible := r.remaining() / minFriendEntrySize
	if int(friendsCount) > maxPossible {
		return fmt.Errorf("friends count %d exceeds maximum possible from remaining data (%d bytes)", friendsCount, r.remaining())
	}

	s.Friends = make(map[uint32]*Friend, friendsCount)
	for i := 0; i < int(friendsCount); i++ {
		friendID, f, err := unmarshalFriendEntry(r)
		if err != nil {
			return err
		}
		s.Friends[friendID] = f
	}
	return nil
}

// unmarshalFriendEntry reads a single friend entry from the snapshot.
func unmarshalFriendEntry(r *snapshotReader) (uint32, *Friend, error) {
	friendID, err := r.readUint32("friend entry")
	if err != nil {
		return 0, nil, err
	}

	pkData, err := r.readBytes(32, "friend public key")
	if err != nil {
		return 0, nil, err
	}
	var pk [32]byte
	copy(pk[:], pkData)

	statusData, err := r.readBytes(2, "friend status")
	if err != nil {
		return 0, nil, err
	}
	status := FriendStatus(statusData[0])
	connStatus := ConnectionStatus(statusData[1])

	fName, err := r.readLengthPrefixedString("friend name")
	if err != nil {
		return 0, nil, err
	}
	fStatus, err := r.readLengthPrefixedString("friend status message")
	if err != nil {
		return 0, nil, err
	}

	lastSeenData, err := r.readBytes(8, "friend last seen")
	if err != nil {
		return 0, nil, err
	}
	lastSeenNano := int64(binary.BigEndian.Uint64(lastSeenData))

	return friendID, &Friend{
		PublicKey:        pk,
		Status:           status,
		ConnectionStatus: connStatus,
		Name:             fName,
		StatusMessage:    fStatus,
		LastSeen:         time.Unix(0, lastSeenNano),
	}, nil
}

// isSnapshotFormat checks if data is in binary snapshot format.
func isSnapshotFormat(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(data[:4]) == SnapshotMagic
}
