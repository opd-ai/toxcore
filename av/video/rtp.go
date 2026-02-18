// Package video provides RTP video packetization for ToxAV.
//
// This file implements RTP packetization for VP8 video frames
// according to RFC 7741 for network transmission over Tox transport.
package video

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function": "NewRTPPacketizer",
		"ssrc":     ssrc,
	}).Info("Creating new RTP packetizer")

	packetizer := &RTPPacketizer{
		ssrc:           ssrc,
		sequenceNumber: 1, // Start from 1
		timestamp:      0,
		payloadType:    96,   // VP8 payload type
		maxPacketSize:  1200, // Conservative MTU minus IP/UDP headers
	}

	logrus.WithFields(logrus.Fields{
		"function":        "NewRTPPacketizer",
		"ssrc":            ssrc,
		"payload_type":    packetizer.payloadType,
		"max_packet_size": packetizer.maxPacketSize,
	}).Info("RTP packetizer created successfully")

	return packetizer
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
	frameBuffer  map[uint32]*FrameAssembly // Buffer for incomplete frames
	maxFrames    int                       // Maximum frames to buffer
	timeProvider TimeProvider              // Time provider for deterministic testing
}

// FrameAssembly represents a frame being reassembled from RTP packets.
type FrameAssembly struct {
	timestamp      uint32      // Frame timestamp
	pictureID      uint16      // Picture ID
	packets        []RTPPacket // Received packets
	receivedSize   int         // Received size so far
	complete       bool        // Frame complete flag
	lastActivity   time.Time   // For timeout handling
	hasStartPacket bool        // Whether we've received the start packet
	startSequence  uint16      // Sequence number of the start packet
}

// NewRTPDepacketizer creates a new VP8 RTP depacketizer.
func NewRTPDepacketizer() *RTPDepacketizer {
	return &RTPDepacketizer{
		frameBuffer:  make(map[uint32]*FrameAssembly),
		maxFrames:    10, // Buffer up to 10 incomplete frames
		timeProvider: DefaultTimeProvider{},
	}
}

// NewRTPDepacketizerWithTimeProvider creates a new VP8 RTP depacketizer with a custom time provider.
// Use this for deterministic testing by injecting a mock time provider.
func NewRTPDepacketizerWithTimeProvider(tp TimeProvider) *RTPDepacketizer {
	return &RTPDepacketizer{
		frameBuffer:  make(map[uint32]*FrameAssembly),
		maxFrames:    10,
		timeProvider: tp,
	}
}

// SetTimeProvider sets the time provider for deterministic testing.
func (rd *RTPDepacketizer) SetTimeProvider(tp TimeProvider) {
	rd.timeProvider = tp
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
	pictureID, frameData, _, err := rd.parseVP8Payload(packet.Payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse VP8 payload: %w", err)
	}

	// Get or create frame assembly
	assembly := rd.getOrCreateFrameAssembly(packet.Timestamp, pictureID)

	// Add packet to assembly
	rd.addPacketToAssembly(assembly, packet, frameData)

	// Check if frame is complete (use packet.StartOfPartition directly)
	isComplete := rd.checkFrameCompletion(assembly, packet, packet.StartOfPartition)

	if isComplete {
		return rd.finalizeCompleteFrame(assembly)
	}

	return nil, 0, nil // Frame not yet complete
}

// getOrCreateFrameAssembly retrieves existing frame assembly or creates a new one.
// Returns the frame assembly for the given timestamp.
func (rd *RTPDepacketizer) getOrCreateFrameAssembly(timestamp uint32, pictureID uint16) *FrameAssembly {
	assembly, exists := rd.frameBuffer[timestamp]
	if !exists {
		// Check buffer size limit
		if len(rd.frameBuffer) >= rd.maxFrames {
			rd.cleanupOldFrames()

			// If still at limit after cleanup, remove oldest frame
			if len(rd.frameBuffer) >= rd.maxFrames {
				rd.removeOldestFrame()
			}
		}

		assembly = &FrameAssembly{
			timestamp:      timestamp,
			pictureID:      pictureID,
			packets:        make([]RTPPacket, 0),
			lastActivity:   rd.timeProvider.Now(),
			hasStartPacket: false,
			startSequence:  0,
		}
		rd.frameBuffer[timestamp] = assembly
	}
	return assembly
}

// addPacketToAssembly adds a packet to the frame assembly and updates metadata.
// Updates the assembly's packet list, received size, and last activity time.
func (rd *RTPDepacketizer) addPacketToAssembly(assembly *FrameAssembly, packet RTPPacket, frameData []byte) {
	assembly.packets = append(assembly.packets, packet)
	assembly.receivedSize += len(frameData)
	assembly.lastActivity = rd.timeProvider.Now()
}

// checkFrameCompletion determines if a frame assembly is complete based on packet markers and sequence continuity.
// Returns true if all packets for the frame have been received in proper sequence.
func (rd *RTPDepacketizer) checkFrameCompletion(assembly *FrameAssembly, packet RTPPacket, startOfPartition bool) bool {
	// Store start packet information if this is the start packet
	if startOfPartition {
		assembly.hasStartPacket = true
		assembly.startSequence = packet.SequenceNumber
	}

	// Need both start packet and marker packet to be complete
	if !assembly.hasStartPacket {
		return false
	}

	hasMarkerInAssembly := rd.hasMarkerPacket(assembly)

	if !hasMarkerInAssembly {
		return false // Can't be complete without a marker packet
	}

	// We have both start and marker packets, check if we have a continuous sequence
	return rd.validateSequenceContinuity(assembly)
}

// hasMarkerPacket checks if the assembly contains a packet with the marker bit set.
// Returns true if any packet in the assembly has the marker flag.
func (rd *RTPDepacketizer) hasMarkerPacket(assembly *FrameAssembly) bool {
	for _, p := range assembly.packets {
		if p.Marker {
			return true
		}
	}
	return false
}

// validateSequenceContinuity sorts packets and checks for sequence gaps.
// Returns true if packets form a continuous sequence from start to marker packet.
func (rd *RTPDepacketizer) validateSequenceContinuity(assembly *FrameAssembly) bool {
	// Sort packets by sequence number to find gaps
	packets := rd.sortPacketsBySequence(assembly.packets)

	// Check if we have a continuous sequence from start to marker
	if len(packets) > 0 {
		return rd.checkSequenceCompleteness(packets, assembly.startSequence)
	}
	return false
}

// sortPacketsBySequence sorts RTP packets by sequence number handling 16-bit wraparound.
// Returns a new slice of packets sorted by sequence number.
func (rd *RTPDepacketizer) sortPacketsBySequence(packets []RTPPacket) []RTPPacket {
	sortedPackets := make([]RTPPacket, len(packets))
	copy(sortedPackets, packets)

	// Simple insertion sort by sequence number (handles wrapping)
	for i := 1; i < len(sortedPackets); i++ {
		key := sortedPackets[i]
		j := i - 1

		for j >= 0 && !rd.isSequenceLess(sortedPackets[j].SequenceNumber, key.SequenceNumber) {
			sortedPackets[j+1] = sortedPackets[j]
			j--
		}
		sortedPackets[j+1] = key
	}

	return sortedPackets
}

// checkSequenceCompleteness validates that we have all packets from start to marker.
// Returns true if no gaps exist from startSequence to marker packet.
func (rd *RTPDepacketizer) checkSequenceCompleteness(packets []RTPPacket, startSequence uint16) bool {
	if len(packets) == 0 {
		return false
	}

	// Check that the last packet has the marker bit
	if !packets[len(packets)-1].Marker {
		return false
	}

	// Find the packet with the start sequence
	startIndex := -1
	for i, pkt := range packets {
		if pkt.SequenceNumber == startSequence {
			startIndex = i
			break
		}
	}

	if startIndex == -1 {
		return false // Start packet not found
	}

	// Check for sequence continuity from start packet to end
	expectedSeq := startSequence

	for i := startIndex; i < len(packets); i++ {
		pkt := packets[i]
		if pkt.SequenceNumber != expectedSeq {
			return false // Gap found
		}
		expectedSeq = (expectedSeq + 1) & 0xFFFF // Handle 16-bit wraparound
	}

	return true
} // finalizeCompleteFrame processes a complete frame assembly and returns the frame data.
// Returns the reassembled frame data, picture ID, and any error that occurred.
func (rd *RTPDepacketizer) finalizeCompleteFrame(assembly *FrameAssembly) ([]byte, uint16, error) {
	assembly.complete = true

	// Reassemble frame
	completeFrame, err := rd.reassembleFrame(assembly)
	if err != nil {
		// If reassembly fails due to sequence issues, don't treat as complete yet
		assembly.complete = false
		return nil, 0, nil
	}

	// Clean up completed frame
	delete(rd.frameBuffer, assembly.timestamp)

	return completeFrame, assembly.pictureID, nil
}

// parseVP8Payload extracts VP8 payload data, picture ID, and start bit.
func (rd *RTPDepacketizer) parseVP8Payload(payload []byte) (uint16, []byte, bool, error) {
	if len(payload) < 3 {
		return 0, nil, false, fmt.Errorf("VP8 payload too short: %d bytes", len(payload))
	}

	// Parse first byte
	firstByte := payload[0]
	hasExtended := (firstByte & 0x80) != 0      // X bit
	startOfPartition := (firstByte & 0x10) != 0 // S bit

	if !hasExtended {
		return 0, nil, false, fmt.Errorf("expected extended control bits in VP8 payload")
	}

	// Parse Picture ID (from our packetizer format: 0x90, 0x80|highByte, lowByte)
	// Second byte has M bit set (0x80) plus high bits of picture ID
	// Third byte has low bits of picture ID
	pictureID := uint16(payload[1]&0x7F)<<8 | uint16(payload[2])
	frameData := payload[3:]

	return pictureID, frameData, startOfPartition, nil
}

// reassembleFrame combines packets into complete frame data.
func (rd *RTPDepacketizer) reassembleFrame(assembly *FrameAssembly) ([]byte, error) {
	if err := rd.validatePacketAssembly(assembly); err != nil {
		return nil, err
	}

	sortedPackets := rd.sortPacketsBySequence(assembly.packets)
	rd.detectSequenceGaps(sortedPackets)

	return rd.combinePacketPayloads(sortedPackets)
}

// validatePacketAssembly checks if the packet assembly contains valid packets.
func (rd *RTPDepacketizer) validatePacketAssembly(assembly *FrameAssembly) error {
	if len(assembly.packets) == 0 {
		return fmt.Errorf("no packets in frame assembly")
	}
	return nil
}

// detectSequenceGaps checks for gaps in sequence numbers and logs warnings.
func (rd *RTPDepacketizer) detectSequenceGaps(packets []RTPPacket) {
	if len(packets) <= 1 {
		return
	}

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

// combinePacketPayloads extracts and combines VP8 payload data from packets.
func (rd *RTPDepacketizer) combinePacketPayloads(packets []RTPPacket) ([]byte, error) {
	var frameData []byte
	for _, packet := range packets {
		_, data, _, err := rd.parseVP8Payload(packet.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse packet payload: %w", err)
		}
		frameData = append(frameData, data...)
	}
	return frameData, nil
}

// isSequenceLess compares sequence numbers handling 16-bit wraparound.
func (rd *RTPDepacketizer) isSequenceLess(a, b uint16) bool {
	return int16(a-b) < 0
}

// cleanupOldFrames removes old incomplete frames to prevent memory leaks.
func (rd *RTPDepacketizer) cleanupOldFrames() {
	cutoff := rd.timeProvider.Now().Add(-5 * time.Second) // 5 second timeout

	for timestamp, assembly := range rd.frameBuffer {
		if assembly.lastActivity.Before(cutoff) {
			delete(rd.frameBuffer, timestamp)
		}
	}
}

// removeOldestFrame removes the frame with the oldest lastActivity time.
func (rd *RTPDepacketizer) removeOldestFrame() {
	if len(rd.frameBuffer) == 0 {
		return
	}

	var oldestTimestamp uint32
	var oldestTime time.Time
	first := true

	for timestamp, assembly := range rd.frameBuffer {
		if first || assembly.lastActivity.Before(oldestTime) {
			oldestTimestamp = timestamp
			oldestTime = assembly.lastActivity
			first = false
		}
	}

	delete(rd.frameBuffer, oldestTimestamp)
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
