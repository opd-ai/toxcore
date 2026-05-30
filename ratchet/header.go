package ratchet

import (
	"encoding/binary"
	"errors"
)

// HeaderSize is the wire size of a marshaled Header in bytes.
const HeaderSize = 32 + 4 + 4 // DHPub + PN + N

// Header carries the per-message ratchet metadata.
//
// On the wire it is encoded as:
//
//	[32]byte  DHPub   – sender's current ratchet public key
//	[4]byte   PN      – previous sending-chain length (big-endian uint32)
//	[4]byte   N       – current message number (big-endian uint32)
type Header struct {
	// DHPub is the sender's current ratchet DH public key.  A new value
	// triggers a DH ratchet step on the receiver.
	DHPub [32]byte
	// PN is the number of messages sent in the previous DH epoch.
	PN uint32
	// N is the zero-based index of this message in the current DH epoch.
	N uint32
}

// Encode marshals the Header into a fixed-length 40-byte slice.
func (h Header) Encode() []byte {
	buf := make([]byte, HeaderSize)
	copy(buf[:32], h.DHPub[:])
	binary.BigEndian.PutUint32(buf[32:36], h.PN)
	binary.BigEndian.PutUint32(buf[36:40], h.N)
	return buf
}

// DecodeHeader unmarshals a Header from buf.
// Returns an error if buf is shorter than HeaderSize bytes.
func DecodeHeader(buf []byte) (Header, error) {
	if len(buf) < HeaderSize {
		return Header{}, errors.New("ratchet: header too short")
	}
	var h Header
	copy(h.DHPub[:], buf[:32])
	h.PN = binary.BigEndian.Uint32(buf[32:36])
	h.N = binary.BigEndian.Uint32(buf[36:40])
	return h, nil
}
