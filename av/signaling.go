package av

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
)

// Signaling protocol defines the ToxAV call signaling messages
// exchanged between peers over the existing Tox transport layer.
//
// This follows the established patterns from toxcore-go:
// - Simple, minimal packet formats
// - Clear separation of concerns
// - Reuse of existing transport infrastructure

// CallRequestPacket represents a call initiation request.
//
// Wire format:
//
//	[CALL_ID(4)][AUDIO_BITRATE(4)][VIDEO_BITRATE(4)][TIMESTAMP(8)]
//
// Total size: 20 bytes
type CallRequestPacket struct {
	CallID       uint32    // Unique call identifier
	AudioBitRate uint32    // Requested audio bit rate (0 = disabled)
	VideoBitRate uint32    // Requested video bit rate (0 = disabled)
	Timestamp    time.Time // Call initiation timestamp
}

// CallResponsePacket represents a call answer.
//
// Wire format:
//
//	[CALL_ID(4)][ACCEPTED(1)][AUDIO_BITRATE(4)][VIDEO_BITRATE(4)][TIMESTAMP(8)]
//
// Total size: 21 bytes
type CallResponsePacket struct {
	CallID       uint32    // Call identifier from request
	Accepted     bool      // Whether call was accepted
	AudioBitRate uint32    // Accepted audio bit rate (0 = disabled)
	VideoBitRate uint32    // Accepted video bit rate (0 = disabled)
	Timestamp    time.Time // Response timestamp
}

// CallControlPacket represents call control messages.
//
// Wire format:
//
//	[CALL_ID(4)][CONTROL_TYPE(1)][TIMESTAMP(8)]
//
// Total size: 13 bytes
type CallControlPacket struct {
	CallID      uint32      // Call identifier
	ControlType CallControl // Control action to perform
	Timestamp   time.Time   // Control message timestamp
}

// BitrateControlPacket represents bitrate change requests.
//
// Wire format:
//
//	[CALL_ID(4)][AUDIO_BITRATE(4)][VIDEO_BITRATE(4)][TIMESTAMP(8)]
//
// Total size: 20 bytes
type BitrateControlPacket struct {
	CallID       uint32    // Call identifier
	AudioBitRate uint32    // New audio bit rate (0 = disabled)
	VideoBitRate uint32    // New video bit rate (0 = disabled)
	Timestamp    time.Time // Bitrate change timestamp
}

// SerializeCallRequest converts a CallRequestPacket to bytes for transmission.
func SerializeCallRequest(req *CallRequestPacket) ([]byte, error) {
	if req == nil {
		logrus.WithFields(logrus.Fields{
			"function": "SerializeCallRequest",
			"error":    "call request packet is nil",
		}).Error("Invalid call request packet")
		return nil, errors.New("call request packet is nil")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "SerializeCallRequest",
		"call_id":       req.CallID,
		"audio_bitrate": req.AudioBitRate,
		"video_bitrate": req.VideoBitRate,
	}).Debug("Serializing call request packet")

	data := make([]byte, 20)
	binary.BigEndian.PutUint32(data[0:4], req.CallID)
	binary.BigEndian.PutUint32(data[4:8], req.AudioBitRate)
	binary.BigEndian.PutUint32(data[8:12], req.VideoBitRate)
	binary.BigEndian.PutUint64(data[12:20], uint64(req.Timestamp.UnixNano()))

	logrus.WithFields(logrus.Fields{
		"function":  "SerializeCallRequest",
		"data_size": len(data),
	}).Debug("Call request packet serialized successfully")

	return data, nil
}

// DeserializeCallRequest converts bytes to a CallRequestPacket.
func DeserializeCallRequest(data []byte) (*CallRequestPacket, error) {
	if len(data) < 20 {
		return nil, errors.New("call request packet too short")
	}

	return &CallRequestPacket{
		CallID:       binary.BigEndian.Uint32(data[0:4]),
		AudioBitRate: binary.BigEndian.Uint32(data[4:8]),
		VideoBitRate: binary.BigEndian.Uint32(data[8:12]),
		Timestamp:    time.Unix(0, int64(binary.BigEndian.Uint64(data[12:20]))),
	}, nil
}

// SerializeCallResponse converts a CallResponsePacket to bytes for transmission.
func SerializeCallResponse(resp *CallResponsePacket) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("call response packet is nil")
	}

	data := make([]byte, 21)
	binary.BigEndian.PutUint32(data[0:4], resp.CallID)

	if resp.Accepted {
		data[4] = 1
	} else {
		data[4] = 0
	}

	binary.BigEndian.PutUint32(data[5:9], resp.AudioBitRate)
	binary.BigEndian.PutUint32(data[9:13], resp.VideoBitRate)
	binary.BigEndian.PutUint64(data[13:21], uint64(resp.Timestamp.UnixNano()))

	return data, nil
}

// DeserializeCallResponse converts bytes to a CallResponsePacket.
func DeserializeCallResponse(data []byte) (*CallResponsePacket, error) {
	if len(data) < 21 {
		return nil, errors.New("call response packet too short")
	}

	return &CallResponsePacket{
		CallID:       binary.BigEndian.Uint32(data[0:4]),
		Accepted:     data[4] != 0,
		AudioBitRate: binary.BigEndian.Uint32(data[5:9]),
		VideoBitRate: binary.BigEndian.Uint32(data[9:13]),
		Timestamp:    time.Unix(0, int64(binary.BigEndian.Uint64(data[13:21]))),
	}, nil
}

// SerializeCallControl converts a CallControlPacket to bytes for transmission.
func SerializeCallControl(ctrl *CallControlPacket) ([]byte, error) {
	if ctrl == nil {
		return nil, errors.New("call control packet is nil")
	}

	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], ctrl.CallID)
	data[4] = byte(ctrl.ControlType)
	binary.BigEndian.PutUint64(data[5:13], uint64(ctrl.Timestamp.UnixNano()))

	return data, nil
}

// DeserializeCallControl converts bytes to a CallControlPacket.
func DeserializeCallControl(data []byte) (*CallControlPacket, error) {
	if len(data) < 13 {
		return nil, errors.New("call control packet too short")
	}

	return &CallControlPacket{
		CallID:      binary.BigEndian.Uint32(data[0:4]),
		ControlType: CallControl(data[4]),
		Timestamp:   time.Unix(0, int64(binary.BigEndian.Uint64(data[5:13]))),
	}, nil
}

// SerializeBitrateControl converts a BitrateControlPacket to bytes for transmission.
func SerializeBitrateControl(ctrl *BitrateControlPacket) ([]byte, error) {
	if ctrl == nil {
		return nil, errors.New("bitrate control packet is nil")
	}

	data := make([]byte, 20)
	binary.BigEndian.PutUint32(data[0:4], ctrl.CallID)
	binary.BigEndian.PutUint32(data[4:8], ctrl.AudioBitRate)
	binary.BigEndian.PutUint32(data[8:12], ctrl.VideoBitRate)
	binary.BigEndian.PutUint64(data[12:20], uint64(ctrl.Timestamp.UnixNano()))

	return data, nil
}

// DeserializeBitrateControl converts bytes to a BitrateControlPacket.
func DeserializeBitrateControl(data []byte) (*BitrateControlPacket, error) {
	if len(data) < 20 {
		return nil, errors.New("bitrate control packet too short")
	}

	return &BitrateControlPacket{
		CallID:       binary.BigEndian.Uint32(data[0:4]),
		AudioBitRate: binary.BigEndian.Uint32(data[4:8]),
		VideoBitRate: binary.BigEndian.Uint32(data[8:12]),
		Timestamp:    time.Unix(0, int64(binary.BigEndian.Uint64(data[12:20]))),
	}, nil
}
