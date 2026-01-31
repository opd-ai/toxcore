package toxcore

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// extractIPBytes extracts IPv4 address bytes from various net.Addr types.
// Supports *net.UDPAddr, *net.TCPAddr, and *net.IPAddr.
// Returns an error for nil addresses, IPv6 addresses, or unsupported types.
func extractIPBytes(addr net.Addr) ([]byte, error) {
	if addr == nil {
		return nil, errors.New("address is nil")
	}

	var ip net.IP

	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
	case *net.TCPAddr:
		ip = a.IP
	case *net.IPAddr:
		ip = a.IP
	default:
		return nil, fmt.Errorf("unsupported address type: %T", addr)
	}

	if ip == nil {
		return nil, errors.New("IP address is nil")
	}

	// Convert to IPv4
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, errors.New("only IPv4 addresses are supported")
	}

	return []byte(ipv4), nil
}

// toxAVTransportAdapter adapts the Tox UDP transport for use with the AV manager.
// This allows the AV manager to use the existing transport infrastructure.
type toxAVTransportAdapter struct {
	udpTransport transport.Transport
}

// newToxAVTransportAdapter creates a new transport adapter for ToxAV.
func newToxAVTransportAdapter(udpTransport transport.Transport) *toxAVTransportAdapter {
	logrus.WithFields(logrus.Fields{
		"function": "newToxAVTransportAdapter",
	}).Debug("Creating new ToxAV transport adapter")

	adapter := &toxAVTransportAdapter{
		udpTransport: udpTransport,
	}

	logrus.WithFields(logrus.Fields{
		"function": "newToxAVTransportAdapter",
	}).Info("ToxAV transport adapter created successfully")

	return adapter
}

// Send implements the TransportInterface for the AV manager.
func (t *toxAVTransportAdapter) Send(packetType byte, data, addr []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":    "Send",
		"packet_type": fmt.Sprintf("0x%02x", packetType),
		"data_size":   len(data),
		"addr_size":   len(addr),
	}).Debug("Sending AV packet via transport adapter")

	// Convert AV packet type to transport PacketType
	var transportPacketType transport.PacketType
	switch packetType {
	case 0x30:
		transportPacketType = transport.PacketAVCallRequest
	case 0x31:
		transportPacketType = transport.PacketAVCallResponse
	case 0x32:
		transportPacketType = transport.PacketAVCallControl
	case 0x33:
		transportPacketType = transport.PacketAVAudioFrame
	case 0x34:
		transportPacketType = transport.PacketAVVideoFrame
	case 0x35:
		transportPacketType = transport.PacketAVBitrateControl
	default:
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"packet_type": fmt.Sprintf("0x%02x", packetType),
		}).Error("Unknown AV packet type")
		return fmt.Errorf("unknown AV packet type: 0x%02x", packetType)
	}

	// Create transport packet
	packet := &transport.Packet{
		PacketType: transportPacketType,
		Data:       data,
	}

	// Deserialize byte address to net.Addr
	// Address format: 4 bytes for IPv4 + 2 bytes for port (big-endian)
	//             or: 16 bytes for IPv6 + 2 bytes for port (big-endian)
	var netAddr net.Addr
	if len(addr) == 6 {
		// IPv4: Extract IP address (first 4 bytes)
		ip := net.IPv4(addr[0], addr[1], addr[2], addr[3])
		// Extract port (next 2 bytes, big-endian)
		port := int(addr[4])<<8 | int(addr[5])

		netAddr = &net.UDPAddr{
			IP:   ip,
			Port: port,
		}
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"net_addr":    netAddr.String(),
			"packet_type": transportPacketType,
			"addr_family": "IPv4",
		}).Debug("Converted IPv4 address and sending packet")
	} else if len(addr) == 18 {
		// IPv6: Extract IP address (first 16 bytes)
		ip := net.IP(addr[0:16])
		// Extract port (next 2 bytes, big-endian)
		port := int(addr[16])<<8 | int(addr[17])

		netAddr = &net.UDPAddr{
			IP:   ip,
			Port: port,
		}
		logrus.WithFields(logrus.Fields{
			"function":    "Send",
			"net_addr":    netAddr.String(),
			"packet_type": transportPacketType,
			"addr_family": "IPv6",
		}).Debug("Converted IPv6 address and sending packet")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":  "Send",
			"addr_size": len(addr),
		}).Error("Invalid address format - expected 6 bytes (IPv4) or 18 bytes (IPv6)")
		return fmt.Errorf("invalid address format: expected 6 bytes (IPv4) or 18 bytes (IPv6), got %d", len(addr))
	}

	err := t.udpTransport.Send(packet, netAddr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Send",
			"error":    err.Error(),
			"addr":     netAddr.String(),
		}).Error("Failed to send packet via UDP transport")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":    "Send",
		"packet_type": transportPacketType,
		"addr":        netAddr.String(),
		"data_size":   len(data),
	}).Info("AV packet sent successfully")

	return nil
}

// RegisterHandler implements the TransportInterface for the AV manager.
func (t *toxAVTransportAdapter) RegisterHandler(packetType byte, handler func([]byte, []byte) error) {
	logrus.WithFields(logrus.Fields{
		"function":    "RegisterHandler",
		"packet_type": fmt.Sprintf("0x%02x", packetType),
	}).Debug("Registering AV packet handler")

	// Convert AV packet type to transport PacketType
	var transportPacketType transport.PacketType
	switch packetType {
	case 0x30:
		transportPacketType = transport.PacketAVCallRequest
	case 0x31:
		transportPacketType = transport.PacketAVCallResponse
	case 0x32:
		transportPacketType = transport.PacketAVCallControl
	case 0x33:
		transportPacketType = transport.PacketAVAudioFrame
	case 0x34:
		transportPacketType = transport.PacketAVVideoFrame
	case 0x35:
		transportPacketType = transport.PacketAVBitrateControl
	default:
		logrus.WithFields(logrus.Fields{
			"function":    "RegisterHandler",
			"packet_type": fmt.Sprintf("0x%02x", packetType),
		}).Warn("Ignoring unknown AV packet type")
		// Ignore unknown packet types
		return
	}

	// Create adapter function to convert transport calls to AV manager calls
	transportHandler := func(packet *transport.Packet, addr net.Addr) error {
		logrus.WithFields(logrus.Fields{
			"function":    "RegisterHandler.wrapper",
			"packet_type": packet.PacketType,
			"data_size":   len(packet.Data),
			"source_addr": addr.String(),
		}).Debug("Processing received AV packet")

		// Convert net.Addr to byte slice using extractIPBytes
		addrBytes, err := extractIPBytes(addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "RegisterHandler.wrapper",
				"addr_type": fmt.Sprintf("%T", addr),
				"error":     err.Error(),
			}).Error("Failed to extract IP bytes from address")
			return fmt.Errorf("address conversion failed: %w", err)
		}

		logrus.WithFields(logrus.Fields{
			"function": "RegisterHandler.wrapper",
			"addr":     addr.String(),
		}).Debug("Converted address to bytes")

		err = handler(packet.Data, addrBytes)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "RegisterHandler.wrapper",
				"error":       err.Error(),
				"packet_type": packet.PacketType,
			}).Error("AV packet handler failed")
			return err
		}

		logrus.WithFields(logrus.Fields{
			"function":    "RegisterHandler.wrapper",
			"packet_type": packet.PacketType,
			"data_size":   len(packet.Data),
		}).Debug("AV packet processed successfully")

		return nil
	}

	t.udpTransport.RegisterHandler(transportPacketType, transportHandler)

	logrus.WithFields(logrus.Fields{
		"function":    "RegisterHandler",
		"packet_type": transportPacketType,
	}).Info("AV packet handler registered successfully")
}

// ToxAV represents an audio/video instance that integrates with a Tox instance.
//
// ToxAV provides the high-level API for audio/video calling functionality.
// It follows the established patterns in toxcore-go:
// - Callback-based event handling
// - Thread-safe operations
// - Integration with existing Tox infrastructure
// - Compatibility with libtoxcore ToxAV API
type ToxAV struct {
	// Core integration
	tox  *Tox
	impl *avpkg.Manager

	// Thread safety
	mu sync.RWMutex

	// Callbacks matching libtoxcore ToxAV API exactly
	callCb         func(friendNumber uint32, audioEnabled, videoEnabled bool)
	callStateCb    func(friendNumber uint32, state avpkg.CallState)
	audioBitRateCb func(friendNumber, bitRate uint32)
	videoBitRateCb func(friendNumber, bitRate uint32)
	audioReceiveCb func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)
	videoReceiveCb func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)
}

// NewToxAV creates a new ToxAV instance from an existing Tox instance.
//
// The ToxAV instance will integrate with the Tox instance's networking,
// crypto, and friend management systems. This follows the established
// pattern of constructor functions in toxcore-go.
//
// Parameters:
//   - tox: The Tox instance to integrate with
//
// Returns:
//   - *ToxAV: The new ToxAV instance
//   - error: Any error that occurred during setup
func NewToxAV(tox *Tox) (*ToxAV, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewToxAV",
	}).Debug("Creating new ToxAV instance")

	if tox == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewToxAV",
			"error":    "tox instance is nil",
		}).Error("Cannot create ToxAV with nil Tox instance")
		return nil, errors.New("tox instance cannot be nil")
	}

	// Check if transport is available (for testing, it might be nil)
	if tox.udpTransport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewToxAV",
			"error":    "tox transport not initialized",
		}).Error("Tox transport is required for ToxAV initialization")
		return nil, errors.New("tox transport is not initialized")
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewToxAV",
	}).Debug("Creating transport adapter for AV manager")

	// Create transport adapter for the AV manager
	transportAdapter := newToxAVTransportAdapter(tox.udpTransport)

	// Create friend lookup function that resolves actual network addresses
	friendLookup := func(friendNumber uint32) ([]byte, error) {
		logrus.WithFields(logrus.Fields{
			"function":      "NewToxAV.friendLookup",
			"friend_number": friendNumber,
		}).Debug("Looking up friend address")

		// Get friend from Tox instance
		tox.friendsMutex.RLock()
		friend, exists := tox.friends[friendNumber]
		tox.friendsMutex.RUnlock()

		if !exists {
			err := fmt.Errorf("friend %d not found", friendNumber)
			logrus.WithFields(logrus.Fields{
				"function":      "NewToxAV.friendLookup",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Error("Friend lookup failed")
			return nil, err
		}

		// Resolve friend's network address via DHT
		addr, err := tox.resolveFriendAddress(friend)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "NewToxAV.friendLookup",
				"friend_number": friendNumber,
				"error":         err.Error(),
			}).Error("Failed to resolve friend address")
			return nil, fmt.Errorf("failed to resolve address for friend %d: %w", friendNumber, err)
		}

		// Serialize net.Addr to bytes using transport abstraction
		// This avoids concrete type assertions and supports future transport types
		addrBytes, err := transport.SerializeNetAddrToBytes(addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":      "NewToxAV.friendLookup",
				"friend_number": friendNumber,
				"addr_type":     fmt.Sprintf("%T", addr),
				"error":         err.Error(),
			}).Error("Failed to serialize address")
			return nil, fmt.Errorf("failed to serialize address for friend %d: %w", friendNumber, err)
		}

		logrus.WithFields(logrus.Fields{
			"function":      "NewToxAV.friendLookup",
			"friend_number": friendNumber,
			"address":       addr.String(),
			"addr_bytes":    fmt.Sprintf("%v", addrBytes),
		}).Debug("Friend address resolved and serialized")

		return addrBytes, nil
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewToxAV",
	}).Debug("Creating AV manager with transport integration")

	// Create the underlying manager with transport integration
	manager, err := avpkg.NewManager(transportAdapter, friendLookup)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewToxAV",
			"error":    err.Error(),
		}).Error("Failed to create AV manager")
		return nil, fmt.Errorf("failed to create AV manager: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewToxAV",
	}).Debug("Starting AV manager")

	// Start the manager
	if err := manager.Start(); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewToxAV",
			"error":    err.Error(),
		}).Error("Failed to start AV manager")
		return nil, fmt.Errorf("failed to start AV manager: %w", err)
	}

	toxav := &ToxAV{
		tox:  tox,
		impl: manager,
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewToxAV",
	}).Info("ToxAV instance created and started successfully")

	return toxav, nil
} // Kill gracefully shuts down the ToxAV instance.
// This method ends all active calls and releases resources.
// It follows the established cleanup patterns in toxcore-go.
func (av *ToxAV) Kill() {
	logrus.WithFields(logrus.Fields{
		"function": "Kill",
	}).Debug("Shutting down ToxAV instance")

	av.mu.Lock()
	defer av.mu.Unlock()

	if av.impl != nil {
		logrus.WithFields(logrus.Fields{
			"function": "Kill",
		}).Debug("Stopping AV manager")

		av.impl.Stop()
		av.impl = nil

		logrus.WithFields(logrus.Fields{
			"function": "Kill",
		}).Info("AV manager stopped and ToxAV instance shut down")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "Kill",
		}).Debug("ToxAV instance already stopped")
	}
}

// Iterate performs one iteration of the ToxAV event loop.
//
// This method should be called regularly (at IterationInterval) to
// process audio/video events and maintain call state. It follows
// the established iteration pattern in toxcore-go.
func (av *ToxAV) Iterate() {
	logrus.WithFields(logrus.Fields{
		"function": "Iterate",
	}).Trace("Performing ToxAV iteration")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl != nil {
		impl.Iterate()
		logrus.WithFields(logrus.Fields{
			"function": "Iterate",
		}).Trace("AV manager iteration completed")
	} else {
		logrus.WithFields(logrus.Fields{
			"function": "Iterate",
		}).Debug("No AV manager available for iteration")
	}
}

// IterationInterval returns the recommended interval for calling Iterate().
//
// This follows the established pattern in toxcore-go where components
// provide their own iteration timing requirements.
func (av *ToxAV) IterationInterval() time.Duration {
	logrus.WithFields(logrus.Fields{
		"function": "IterationInterval",
	}).Trace("Getting ToxAV iteration interval")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl != nil {
		interval := impl.IterationInterval()
		logrus.WithFields(logrus.Fields{
			"function": "IterationInterval",
			"interval": interval.String(),
		}).Trace("Returning AV manager iteration interval")
		return interval
	}

	defaultInterval := 20 * time.Millisecond
	logrus.WithFields(logrus.Fields{
		"function": "IterationInterval",
		"interval": defaultInterval.String(),
	}).Debug("Returning default iteration interval (no AV manager)")
	return defaultInterval
}

// Call initiates an audio/video call to a friend.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call initiation
func (av *ToxAV) Call(friendNumber, audioBitRate, videoBitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "Call",
		"friend_number": friendNumber,
		"audio_bitrate": audioBitRate,
		"video_bitrate": videoBitRate,
		"audio_enabled": audioBitRate > 0,
		"video_enabled": videoBitRate > 0,
	}).Info("Initiating call to friend")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "Call",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot initiate call - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	err := impl.StartCall(friendNumber, audioBitRate, videoBitRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "Call",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to start call")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":      "Call",
		"friend_number": friendNumber,
		"audio_bitrate": audioBitRate,
		"video_bitrate": videoBitRate,
	}).Info("Call initiated successfully")

	return nil
}

// Answer accepts an incoming audio/video call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend who initiated the call
//   - audioBitRate: Audio bit rate in bits/second (0 to disable audio)
//   - videoBitRate: Video bit rate in bits/second (0 to disable video)
//
// Returns:
//   - error: Any error that occurred during call answer
func (av *ToxAV) Answer(friendNumber, audioBitRate, videoBitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "Answer",
		"friend_number": friendNumber,
		"audio_bitrate": audioBitRate,
		"video_bitrate": videoBitRate,
		"audio_enabled": audioBitRate > 0,
		"video_enabled": videoBitRate > 0,
	}).Info("Answering incoming call")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "Answer",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot answer call - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	err := impl.AnswerCall(friendNumber, audioBitRate, videoBitRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "Answer",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to answer call")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":      "Answer",
		"friend_number": friendNumber,
		"audio_bitrate": audioBitRate,
		"video_bitrate": videoBitRate,
	}).Info("Call answered successfully")

	return nil
}

// CallControl sends a call control command.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the control command to
//   - control: The control command to send
//
// Returns:
//   - error: Any error that occurred during control command sending
func (av *ToxAV) CallControl(friendNumber uint32, control avpkg.CallControl) error {
	logrus.WithFields(logrus.Fields{
		"function":      "CallControl",
		"friend_number": friendNumber,
		"control":       control.String(),
		"control_code":  int(control),
	}).Info("Sending call control command")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot send call control - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	var err error
	switch control {
	case avpkg.CallControlCancel:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "ending_call",
		}).Debug("Ending call")
		err = impl.EndCall(friendNumber)
	case avpkg.CallControlResume:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "resuming_call",
		}).Debug("Resuming call")
		err = impl.ResumeCall(friendNumber)
	case avpkg.CallControlPause:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "pausing_call",
		}).Debug("Pausing call")
		err = impl.PauseCall(friendNumber)
	case avpkg.CallControlMuteAudio:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "muting_audio",
		}).Debug("Muting audio")
		err = impl.MuteAudio(friendNumber)
	case avpkg.CallControlUnmuteAudio:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "unmuting_audio",
		}).Debug("Unmuting audio")
		err = impl.UnmuteAudio(friendNumber)
	case avpkg.CallControlHideVideo:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "hiding_video",
		}).Debug("Hiding video")
		err = impl.HideVideo(friendNumber)
	case avpkg.CallControlShowVideo:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"action":        "showing_video",
		}).Debug("Showing video")
		err = impl.ShowVideo(friendNumber)
	default:
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"control_code":  int(control),
		}).Error("Unknown call control command")
		err = fmt.Errorf("unknown call control: %d", control)
	}

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "CallControl",
			"friend_number": friendNumber,
			"control":       control.String(),
			"error":         err.Error(),
		}).Error("Call control command failed")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function":      "CallControl",
		"friend_number": friendNumber,
		"control":       control.String(),
	}).Info("Call control command executed successfully")

	return nil
}

// AudioSetBitRate sets the audio bit rate for an active call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to update audio bit rate for
//   - bitRate: New audio bit rate in bits/second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (av *ToxAV) AudioSetBitRate(friendNumber, bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "AudioSetBitRate",
		"friend_number": friendNumber,
		"bitrate":       bitRate,
	}).Info("Setting audio bit rate for call")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSetBitRate",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot set audio bit rate - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	// For Phase 1, send a bitrate control packet to adjust audio bitrate
	call := impl.GetCall(friendNumber)
	if call == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSetBitRate",
			"friend_number": friendNumber,
		}).Error("No active call found with this friend")
		return errors.New("no active call with this friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "AudioSetBitRate",
		"friend_number": friendNumber,
		"old_bitrate":   call.GetAudioBitRate(),
		"new_bitrate":   bitRate,
	}).Debug("Updating call audio bit rate")

	// Update the call's audio bitrate (this is a simplified implementation)
	call.SetAudioBitRate(bitRate)

	logrus.WithFields(logrus.Fields{
		"function":      "AudioSetBitRate",
		"friend_number": friendNumber,
		"bitrate":       bitRate,
	}).Info("Audio bit rate updated successfully")

	// In a full implementation, this would send a BitrateControlPacket
	// For Phase 1, we'll just update the local state
	return nil
}

// VideoSetBitRate sets the video bit rate for an active call.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to update video bit rate for
//   - bitRate: New video bit rate in bits/second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (av *ToxAV) VideoSetBitRate(friendNumber, bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "VideoSetBitRate",
		"friend_number": friendNumber,
		"bitrate":       bitRate,
	}).Info("Setting video bit rate for call")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSetBitRate",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot set video bit rate - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	// For Phase 1, send a bitrate control packet to adjust video bitrate
	call := impl.GetCall(friendNumber)
	if call == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSetBitRate",
			"friend_number": friendNumber,
		}).Error("No active call found with this friend")
		return errors.New("no active call with this friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "VideoSetBitRate",
		"friend_number": friendNumber,
		"old_bitrate":   call.GetVideoBitRate(),
		"new_bitrate":   bitRate,
	}).Debug("Updating call video bit rate")

	// Update the call's video bitrate (this is a simplified implementation)
	call.SetVideoBitRate(bitRate)

	logrus.WithFields(logrus.Fields{
		"function":      "VideoSetBitRate",
		"friend_number": friendNumber,
		"bitrate":       bitRate,
	}).Info("Video bit rate updated successfully")

	// In a full implementation, this would send a BitrateControlPacket
	// For Phase 1, we'll just update the local state
	return nil
}

// AudioSendFrame sends an audio frame to a friend.
//
// This method implements audio frame sending by integrating the completed
// audio processing pipeline and RTP packetization system. The audio data
// is processed through the audio processor and sent via RTP transport.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the audio frame to
//   - pcm: PCM audio data as signed 16-bit samples
//   - sampleCount: Number of audio samples per channel
//   - channels: Number of audio channels (1 or 2)
//   - samplingRate: Audio sampling rate in Hz
//
// Returns:
//   - error: Any error that occurred during frame sending
func (av *ToxAV) AudioSendFrame(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":      "AudioSendFrame",
		"friend_number": friendNumber,
		"pcm_length":    len(pcm),
		"sample_count":  sampleCount,
		"channels":      channels,
		"sampling_rate": samplingRate,
	}).Trace("Sending audio frame")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot send audio frame - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	// Validate parameters
	if len(pcm) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
		}).Error("Empty PCM data provided")
		return errors.New("empty PCM data")
	}

	if sampleCount <= 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
			"sample_count":  sampleCount,
		}).Error("Invalid sample count")
		return errors.New("invalid sample count")
	}

	if channels == 0 || channels > 2 {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
			"channels":      channels,
		}).Error("Invalid channel count")
		return errors.New("invalid channel count (must be 1 or 2)")
	}

	if samplingRate == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
			"sampling_rate": samplingRate,
		}).Error("Invalid sampling rate")
		return errors.New("invalid sampling rate")
	}

	// Get the active call for this friend
	call := impl.GetCall(friendNumber)
	if call == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
		}).Error("No active call found with friend")
		return errors.New("no active call with this friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "AudioSendFrame",
		"friend_number": friendNumber,
		"sample_count":  sampleCount,
		"channels":      channels,
		"data_size":     len(pcm) * 2, // int16 = 2 bytes
	}).Debug("Delegating audio frame to call handler")

	// Delegate to the call's audio frame sending method
	// This integrates the completed audio processing and RTP packetization
	err := call.SendAudioFrame(pcm, sampleCount, channels, samplingRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioSendFrame",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to send audio frame")
		return fmt.Errorf("failed to send audio frame: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "AudioSendFrame",
		"friend_number": friendNumber,
		"sample_count":  sampleCount,
	}).Trace("Audio frame sent successfully")

	return nil
}

// VideoSendFrame sends a video frame to a friend.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - friendNumber: The friend to send the video frame to
//   - width: Video frame width in pixels
//   - height: Video frame height in pixels
//   - y: Y plane data (luminance)
//   - u: U plane data (chrominance)
//   - v: V plane data (chrominance)
//
// Returns:
//   - error: Any error that occurred during frame sending
func (av *ToxAV) VideoSendFrame(friendNumber uint32, width, height uint16, y, u, v []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":      "VideoSendFrame",
		"friend_number": friendNumber,
		"width":         width,
		"height":        height,
		"y_size":        len(y),
		"u_size":        len(u),
		"v_size":        len(v),
	}).Debug("Attempting to send video frame")

	av.mu.RLock()
	impl := av.impl
	av.mu.RUnlock()

	if impl == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSendFrame",
			"friend_number": friendNumber,
			"error":         "ToxAV instance destroyed",
		}).Error("Cannot send video frame - ToxAV instance has been destroyed")
		return errors.New("ToxAV instance has been destroyed")
	}

	// Validate input parameters
	if width == 0 || height == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSendFrame",
			"friend_number": friendNumber,
			"width":         width,
			"height":        height,
		}).Error("Invalid video frame dimensions")
		return fmt.Errorf("invalid frame dimensions: %dx%d", width, height)
	}

	if len(y) == 0 || len(u) == 0 || len(v) == 0 {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSendFrame",
			"friend_number": friendNumber,
			"y_size":        len(y),
			"u_size":        len(u),
			"v_size":        len(v),
		}).Error("Empty video plane data provided")
		return errors.New("video plane data cannot be empty")
	}

	// Get the active call for this friend
	call := impl.GetCall(friendNumber)
	if call == nil {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSendFrame",
			"friend_number": friendNumber,
		}).Error("No active call found with friend")
		return errors.New("no active call with this friend")
	}

	logrus.WithFields(logrus.Fields{
		"function":      "VideoSendFrame",
		"friend_number": friendNumber,
		"width":         width,
		"height":        height,
		"y_size":        len(y),
		"u_size":        len(u),
		"v_size":        len(v),
	}).Debug("Delegating video frame to call handler")

	// Delegate to the call's video frame sending method
	// This integrates the video processing pipeline with RTP packetization
	err := call.SendVideoFrame(width, height, y, u, v)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "VideoSendFrame",
			"friend_number": friendNumber,
			"error":         err.Error(),
		}).Error("Failed to send video frame")
		return fmt.Errorf("failed to send video frame: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "VideoSendFrame",
		"friend_number": friendNumber,
		"width":         width,
		"height":        height,
	}).Info("Video frame sent successfully")

	return nil
}

// CallbackCall sets the callback for incoming call requests.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when a call request is received
func (av *ToxAV) CallbackCall(callback func(friendNumber uint32, audioEnabled, videoEnabled bool)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackCall",
		"callback_is_nil": callback == nil,
	}).Debug("Setting call request callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.callCb = callback

	// Wire the callback to the underlying av.Manager
	if av.impl != nil {
		av.impl.SetCallCallback(callback)
		logrus.WithFields(logrus.Fields{
			"function": "CallbackCall",
		}).Debug("Call callback wired to av.Manager")
	}

	logrus.WithFields(logrus.Fields{
		"function": "CallbackCall",
	}).Info("Call request callback registered")
}

// CallbackCallState sets the callback for call state changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when call state changes
func (av *ToxAV) CallbackCallState(callback func(friendNumber uint32, state avpkg.CallState)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackCallState",
		"callback_is_nil": callback == nil,
	}).Debug("Setting call state change callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.callStateCb = callback

	// Wire the callback to the underlying av.Manager
	if av.impl != nil {
		av.impl.SetCallStateCallback(callback)
		logrus.WithFields(logrus.Fields{
			"function": "CallbackCallState",
		}).Debug("Call state callback wired to av.Manager")
	}

	logrus.WithFields(logrus.Fields{
		"function": "CallbackCallState",
	}).Info("Call state change callback registered")
}

// CallbackAudioBitRate sets the callback for audio bit rate changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when audio bit rate changes
func (av *ToxAV) CallbackAudioBitRate(callback func(friendNumber, bitRate uint32)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackAudioBitRate",
		"callback_is_nil": callback == nil,
	}).Debug("Setting audio bit rate change callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.audioBitRateCb = callback

	logrus.WithFields(logrus.Fields{
		"function": "CallbackAudioBitRate",
	}).Info("Audio bit rate change callback registered")
}

// CallbackVideoBitRate sets the callback for video bit rate changes.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when video bit rate changes
func (av *ToxAV) CallbackVideoBitRate(callback func(friendNumber, bitRate uint32)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackVideoBitRate",
		"callback_is_nil": callback == nil,
	}).Debug("Setting video bit rate change callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.videoBitRateCb = callback

	logrus.WithFields(logrus.Fields{
		"function": "CallbackVideoBitRate",
	}).Info("Video bit rate change callback registered")
}

// CallbackAudioReceiveFrame sets the callback for incoming audio frames.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when an audio frame is received
func (av *ToxAV) CallbackAudioReceiveFrame(callback func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackAudioReceiveFrame",
		"callback_is_nil": callback == nil,
	}).Debug("Setting audio frame receive callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.audioReceiveCb = callback

	// Wire the callback to the underlying av.Manager
	if av.impl != nil {
		av.impl.SetAudioReceiveCallback(callback)
		logrus.WithFields(logrus.Fields{
			"function": "CallbackAudioReceiveFrame",
		}).Debug("Audio callback wired to av.Manager")
	}

	logrus.WithFields(logrus.Fields{
		"function": "CallbackAudioReceiveFrame",
	}).Info("Audio frame receive callback registered")
}

// CallbackVideoReceiveFrame sets the callback for incoming video frames.
//
// This method matches the libtoxcore ToxAV API exactly for compatibility.
//
// Parameters:
//   - callback: Function to call when a video frame is received
func (av *ToxAV) CallbackVideoReceiveFrame(callback func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int)) {
	logrus.WithFields(logrus.Fields{
		"function":        "CallbackVideoReceiveFrame",
		"callback_is_nil": callback == nil,
	}).Debug("Setting video frame receive callback")

	av.mu.Lock()
	defer av.mu.Unlock()
	av.videoReceiveCb = callback

	// Wire the callback to the underlying av.Manager
	if av.impl != nil {
		av.impl.SetVideoReceiveCallback(callback)
		logrus.WithFields(logrus.Fields{
			"function": "CallbackVideoReceiveFrame",
		}).Debug("Video callback wired to av.Manager")
	}

	logrus.WithFields(logrus.Fields{
		"function": "CallbackVideoReceiveFrame",
	}).Info("Video frame receive callback registered")
}
