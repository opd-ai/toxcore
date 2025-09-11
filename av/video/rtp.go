// Package video provides RTP video packetization for ToxAV.
//
// This file implements RTP packetization for VP8 video frames
// according to RFC 7741 for network transmission over Tox transport.
package video

import (
	"fmt"
	"time"
)

// RTPPacket represents an RTP packet for video transmission.
type RTPPacket struct {
	// RTP Header
	Version        uint8  // Always 2 for RTP
	Padding        bool   // Padding flag
	Extension      bool   // Extension flag
	CSRCCount      uint8  // CSRC count
	Marker         bool   // Marker bit (end of frame)
	PayloadType    uint8  // Payload type (VP8 = 96)
	SequenceNumber uint16 // Sequence number
	Timestamp      uint32 // Timestamp
	SSRC           uint32 // Synchronization source identifier

	// VP8 Payload Descriptor (RFC 7741)
	ExtendedControlBits bool   // X bit
	NonReferenceBit     bool   // N bit
	StartOfPartition    bool   // S bit
	PictureID           uint16 // Picture ID

	// Payload data
	Payload []byte // VP8 encoded data
}

// RTPPacketizer handles VP8 frame packetization into RTP packets.
type RTPPacketizer struct {
	ssrc           uint32 // Source identifier
	sequenceNumber uint16 // Current sequence number
	timestamp      uint32 // Current timestamp
	payloadType    uint8  // RTP payload type for VP8
	maxPacketSize  int    // Maximum packet size (MTU consideration)
}

// NewRTPPacketizer creates a new VP8 RTP packetizer.
func NewRTPPacketizer(ssrc uint32) *RTPPacketizer {
	return &RTPPacketizer{
		ssrc:           ssrc,
		sequenceNumber: 1, // Start from 1
		timestamp:      0,
		payloadType:    96,   // VP8 payload type
		maxPacketSize:  1200, // Conservative MTU minus IP/UDP headers
	}
}

// PacketizeFrame converts a VP8 encoded frame into RTP packets.
//
// Splits large frames into multiple RTP packets while maintaining
// VP8 payload descriptor structure according to RFC 7741.
//
// Parameters:
//   - frameData: VP8 encoded frame data
//   - frameTimestamp: Frame timestamp (typically based on 90kHz clock)
//   - pictureID: VP8 picture ID for frame identification
//
// Returns:
//   - []RTPPacket: Array of RTP packets for the frame
//   - error: Any error that occurred during packetization
func (rp *RTPPacketizer) PacketizeFrame(frameData []byte, frameTimestamp uint32, pictureID uint16) ([]RTPPacket, error) {
	if len(frameData) == 0 {
		return nil, fmt.Errorf("frame data cannot be empty")
	}

	if len(frameData) > 2000000 { // 2MB limit for large frames
		return nil, fmt.Errorf("frame data too large: %d bytes (max 2000000)", len(frameData))
	}

	// Calculate VP8 payload descriptor size (3 bytes for basic descriptor + PictureID)
	descriptorSize := 3                                      // X bit + basic descriptor + PictureID (2 bytes)
	maxPayloadSize := rp.maxPacketSize - 12 - descriptorSize // RTP header (12) + VP8 descriptor

	if maxPayloadSize <= 0 {
		return nil, fmt.Errorf("max packet size too small: %d", rp.maxPacketSize)
	}

	// Calculate number of packets needed
	numPackets := (len(frameData) + maxPayloadSize - 1) / maxPayloadSize
	packets := make([]RTPPacket, numPackets)

	for i := 0; i < numPackets; i++ {
		// Calculate payload slice for this packet
		start := i * maxPayloadSize
		end := start + maxPayloadSize
		if end > len(frameData) {
			end = len(frameData)
		}

		// Create RTP packet
		packet := RTPPacket{
			Version:        2,
			Padding:        false,
			Extension:      false,
			CSRCCount:      0,
			Marker:         i == numPackets-1, // Mark last packet
			PayloadType:    rp.payloadType,
			SequenceNumber: rp.sequenceNumber,
			Timestamp:      frameTimestamp,
			SSRC:           rp.ssrc,

			// VP8 Payload Descriptor
			ExtendedControlBits: true,   // X bit set
			NonReferenceBit:     false,  // N bit (reference frame)
			StartOfPartition:    i == 0, // S bit (first packet only)
			PictureID:           pictureID,
		}

		// Add VP8 payload descriptor and frame data
		packet.Payload = rp.buildVP8Payload(packet, frameData[start:end])
		packets[i] = packet

		// Increment sequence number for next packet
		rp.sequenceNumber++
		if rp.sequenceNumber == 0 { // Handle overflow
			rp.sequenceNumber = 1
		}
	}

	return packets, nil
}

// buildVP8Payload constructs the VP8 payload with descriptor.
func (rp *RTPPacketizer) buildVP8Payload(packet RTPPacket, frameData []byte) []byte {
	// VP8 Payload Descriptor (RFC 7741)
	payload := make([]byte, 3+len(frameData)) // 3 bytes descriptor + data

	// First byte: X|R|R|R|N|S|PID
	firstByte := byte(0)
	if packet.ExtendedControlBits {
		firstByte |= 0x80 // X bit
	}
	if packet.NonReferenceBit {
		firstByte |= 0x20 // N bit
	}
	if packet.StartOfPartition {
		firstByte |= 0x10 // S bit
	}
	// PID (lower 4 bits) - not used in basic implementation
	payload[0] = firstByte

	// Second and third bytes: Picture ID (I bit + 15-bit Picture ID)
	payload[1] = 0x80 | byte((packet.PictureID>>8)&0x7F) // I bit + upper 7 bits
	payload[2] = byte(packet.PictureID & 0xFF)           // Lower 8 bits

	// Copy frame data
	copy(payload[3:], frameData)

	return payload
}

// RTPDepacketizer handles VP8 frame reassembly from RTP packets.
type RTPDepacketizer struct {
	frameBuffer map[uint32]*FrameAssembly // Buffer for incomplete frames
	maxFrames   int                       // Maximum frames to buffer
}

// FrameAssembly represents a frame being reassembled from RTP packets.
type FrameAssembly struct {
	timestamp    uint32      // Frame timestamp
	pictureID    uint16      // Picture ID
	packets      []RTPPacket // Received packets
	totalSize    int         // Expected total size
	receivedSize int         // Received size so far
	complete     bool        // Frame complete flag
	lastActivity time.Time   // For timeout handling
}

// NewRTPDepacketizer creates a new VP8 RTP depacketizer.
func NewRTPDepacketizer() *RTPDepacketizer {
	return &RTPDepacketizer{
		frameBuffer: make(map[uint32]*FrameAssembly),
		maxFrames:   10, // Buffer up to 10 incomplete frames
	}
}

// ProcessPacket processes an incoming RTP packet and attempts frame reassembly.
//
// Returns complete frames when all packets have been received.
//
// Parameters:
//   - packet: Incoming RTP packet
//
// Returns:
//   - []byte: Complete frame data (nil if frame not yet complete)
//   - uint16: Picture ID of complete frame
//   - error: Any error that occurred during processing
func (rd *RTPDepacketizer) ProcessPacket(packet RTPPacket) ([]byte, uint16, error) {
	// Parse VP8 payload descriptor
	pictureID, frameData, err := rd.parseVP8Payload(packet.Payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse VP8 payload: %v", err)
	} // Get or create frame assembly
	assembly, exists := rd.frameBuffer[packet.Timestamp]
	if !exists {
		// Check buffer size limit
		if len(rd.frameBuffer) >= rd.maxFrames {
			rd.cleanupOldFrames()
		}

		assembly = &FrameAssembly{
			timestamp:    packet.Timestamp,
			pictureID:    pictureID,
			packets:      make([]RTPPacket, 0),
			lastActivity: time.Now(),
		}
		rd.frameBuffer[packet.Timestamp] = assembly
	}

	// Add packet to assembly
	assembly.packets = append(assembly.packets, packet)
	assembly.receivedSize += len(frameData)
	assembly.lastActivity = time.Now()

	// Check if frame is complete
	isMarkerPacket := packet.Marker
	isComplete := false

	fmt.Printf("DEBUG: ProcessPacket called with seq=%d, marker=%t\n", packet.SequenceNumber, packet.Marker)

	// Check for completion if this is a marker packet OR if we already have a marker packet
	hasMarkerInAssembly := false
	for _, p := range assembly.packets {
		if p.Marker {
			hasMarkerInAssembly = true
			break
		}
	}

	fmt.Printf("DEBUG: isMarkerPacket=%t, hasMarkerInAssembly=%t\n", isMarkerPacket, hasMarkerInAssembly)

	if isMarkerPacket || hasMarkerInAssembly {
		// We have a marker packet, but need to check if we have a continuous sequence
		// Sort packets by sequence number to find gaps
		packets := make([]RTPPacket, len(assembly.packets))
		copy(packets, assembly.packets)

		fmt.Printf("DEBUG: Before sorting, packets: ")
		for _, p := range packets {
			fmt.Printf("seq=%d ", p.SequenceNumber)
		}
		fmt.Printf("\n")

		// Simple insertion sort by sequence number (handles wrapping)
		for i := 1; i < len(packets); i++ {
			key := packets[i]
			j := i - 1

			for j >= 0 && !rd.isSequenceLess(packets[j].SequenceNumber, key.SequenceNumber) {
				packets[j+1] = packets[j]
				j--
			}
			packets[j+1] = key
		}

		fmt.Printf("DEBUG: After sorting, packets: ")
		for _, p := range packets {
			fmt.Printf("seq=%d ", p.SequenceNumber)
		}
		fmt.Printf("\n")

		// Check if we have a continuous sequence from first to last (marker)
		if len(packets) > 0 {
			expectedSeq := packets[0].SequenceNumber
			hasGap := false

			fmt.Printf("DEBUG: Checking sequence continuity starting from %d\n", expectedSeq)
			for i, pkt := range packets {
				if i == 0 {
					continue
				}
				expectedSeq++
				fmt.Printf("DEBUG: Expected %d, got %d\n", expectedSeq, pkt.SequenceNumber)
				if pkt.SequenceNumber != expectedSeq {
					hasGap = true
					break
				}
			}

			fmt.Printf("DEBUG: hasGap=%t, lastPacketMarker=%t\n", hasGap, packets[len(packets)-1].Marker)
			// Only complete if no gaps and last packet has marker
			if !hasGap && packets[len(packets)-1].Marker {
				isComplete = true
			}
		}
	}

	if isComplete {
		assembly.complete = true

		// Reassemble frame
		completeFrame, err := rd.reassembleFrame(assembly)
		if err != nil {
			// If reassembly fails due to sequence issues, don't treat as complete yet
			assembly.complete = false
			return nil, 0, nil
		}

		// Clean up completed frame
		delete(rd.frameBuffer, packet.Timestamp)

		return completeFrame, assembly.pictureID, nil
	}

	return nil, 0, nil // Frame not yet complete
}

// parseVP8Payload extracts VP8 payload data and picture ID.
func (rd *RTPDepacketizer) parseVP8Payload(payload []byte) (uint16, []byte, error) {
	if len(payload) < 3 {
		return 0, nil, fmt.Errorf("VP8 payload too short: %d bytes", len(payload))
	}

	// Parse first byte
	firstByte := payload[0]
	hasExtended := (firstByte & 0x80) != 0 // X bit

	if !hasExtended {
		return 0, nil, fmt.Errorf("expected extended control bits in VP8 payload")
	}

	// Parse Picture ID (from our packetizer format: 0x90, 0x80|highByte, lowByte)
	// Second byte has M bit set (0x80) plus high bits of picture ID
	// Third byte has low bits of picture ID
	pictureID := uint16(payload[1]&0x7F)<<8 | uint16(payload[2])
	frameData := payload[3:]

	return pictureID, frameData, nil
}

// reassembleFrame combines packets into complete frame data.
func (rd *RTPDepacketizer) reassembleFrame(assembly *FrameAssembly) ([]byte, error) {
	if len(assembly.packets) == 0 {
		return nil, fmt.Errorf("no packets in frame assembly")
	}

	// Sort packets by sequence number
	packets := make([]RTPPacket, len(assembly.packets))
	copy(packets, assembly.packets)

	// Simple insertion sort by sequence number (handles wrapping)
	for i := 1; i < len(packets); i++ {
		key := packets[i]
		j := i - 1

		for j >= 0 && !rd.isSequenceLess(packets[j].SequenceNumber, key.SequenceNumber) {
			packets[j+1] = packets[j]
			j--
		}
		packets[j+1] = key
	}

	// Check for gaps in sequence numbers (warn but don't fail)
	if len(packets) > 1 {
		expectedSeq := packets[0].SequenceNumber
		for i, packet := range packets {
			if i == 0 {
				continue
			}
			expectedSeq++
			if packet.SequenceNumber != expectedSeq {
				// Log warning but continue - some applications might handle partial frames
				_ = fmt.Sprintf("sequence gap detected: expected %d, got %d", expectedSeq, packet.SequenceNumber)
			}
		}
	}

	// Reassemble frame data
	var frameData []byte
	for _, packet := range packets {
		_, data, err := rd.parseVP8Payload(packet.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse packet payload: %v", err)
		}
		frameData = append(frameData, data...)
	}

	return frameData, nil
}

// isSequenceLess compares sequence numbers handling 16-bit wraparound.
func (rd *RTPDepacketizer) isSequenceLess(a, b uint16) bool {
	return int16(a-b) < 0
}

// sequenceDistance calculates the distance between two sequence numbers handling wraparound
func (rd *RTPDepacketizer) sequenceDistance(start, end uint16) uint16 {
	if end >= start {
		return end - start
	}
	// Handle wraparound: max uint16 is 65535
	return (65535 - start + 1) + end
}

// isFrameComplete checks if a frame assembly has all packets from start to marker
func (rd *RTPDepacketizer) isFrameComplete(assembly *FrameAssembly) bool {
	if len(assembly.packets) == 0 {
		return false
	}

	// Find start packet (S=1) and marker packet
	var startSeq, markerSeq uint16
	var hasStart, hasMarker bool

	for _, packet := range assembly.packets {
		// Check for marker
		if packet.Marker {
			markerSeq = packet.SequenceNumber
			hasMarker = true
		}

		// Check for start packet by parsing first byte of payload
		if len(packet.Payload) > 0 {
			firstByte := packet.Payload[0]
			startOfPartition := (firstByte & 0x10) != 0 // S bit
			if startOfPartition {
				startSeq = packet.SequenceNumber
				hasStart = true
			}
		}
	}

	if !hasStart || !hasMarker {
		return false
	}

	// Calculate expected number of packets
	expectedCount := rd.sequenceDistance(startSeq, markerSeq) + 1

	// Check if we have the right number of packets
	return len(assembly.packets) == int(expectedCount)
}

// cleanupOldFrames removes old incomplete frames to prevent memory leaks.
func (rd *RTPDepacketizer) cleanupOldFrames() {
	cutoff := time.Now().Add(-5 * time.Second) // 5 second timeout

	for timestamp, assembly := range rd.frameBuffer {
		if assembly.lastActivity.Before(cutoff) {
			delete(rd.frameBuffer, timestamp)
		}
	}
}

// GetStats returns packetizer statistics.
func (rp *RTPPacketizer) GetStats() (sequenceNumber uint16, timestamp uint32) {
	return rp.sequenceNumber, rp.timestamp
}

// SetMaxPacketSize configures the maximum RTP packet size.
func (rp *RTPPacketizer) SetMaxPacketSize(size int) error {
	if size < 100 || size > 9000 {
		return fmt.Errorf("invalid packet size: %d (must be 100-9000)", size)
	}
	rp.maxPacketSize = size
	return nil
}

// GetBufferedFrameCount returns the number of frames being assembled.
func (rd *RTPDepacketizer) GetBufferedFrameCount() int {
	return len(rd.frameBuffer)
}
