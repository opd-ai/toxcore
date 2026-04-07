package async

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrInvalidAnnouncementData is returned when announcement data is malformed.
var ErrInvalidAnnouncementData = errors.New("invalid storage node announcement data")

// StorageNodeAnnouncement represents a storage node's announcement for DHT discovery.
type StorageNodeAnnouncement struct {
	PublicKey [32]byte      `json:"public_key"`
	Address   string        `json:"address"`
	Port      uint16        `json:"port"`
	Capacity  uint32        `json:"capacity"` // Estimated message capacity
	Load      uint8         `json:"load"`     // Current load percentage (0-100)
	Timestamp time.Time     `json:"timestamp"`
	TTL       time.Duration `json:"ttl"`
}

// IsExpired returns true if the announcement has exceeded its TTL.
func (a *StorageNodeAnnouncement) IsExpired() bool {
	return time.Since(a.Timestamp) > a.TTL
}

// ToNetAddr converts the announcement to a net.Addr.
func (a *StorageNodeAnnouncement) ToNetAddr() net.Addr {
	return &storageNodeAddr{
		network: "udp",
		address: a.Address,
		port:    a.Port,
	}
}

// storageNodeAddr implements net.Addr for storage nodes.
type storageNodeAddr struct {
	network string
	address string
	port    uint16
}

func (a *storageNodeAddr) Network() string { return a.network }
func (a *storageNodeAddr) String() string {
	return net.JoinHostPort(a.address, string(rune(a.port)))
}

// StorageNodeDiscovery manages discovery and caching of storage nodes.
type StorageNodeDiscovery struct {
	mu              sync.RWMutex
	announcements   map[[32]byte]*StorageNodeAnnouncement
	discoveryActive bool
	selfPublicKey   [32]byte
	selfAddr        string
	selfPort        uint16
	selfCapacity    uint32
	isStorageNode   bool

	// Discovery settings
	discoveryInterval time.Duration
	minCachedNodes    int
	maxCachedNodes    int
	defaultTTL        time.Duration

	// Callback for discovered nodes
	onNodeDiscovered func(announcement *StorageNodeAnnouncement)
}

// NewStorageNodeDiscovery creates a new storage node discovery manager.
func NewStorageNodeDiscovery() *StorageNodeDiscovery {
	return &StorageNodeDiscovery{
		announcements:     make(map[[32]byte]*StorageNodeAnnouncement),
		discoveryInterval: 5 * time.Minute,
		minCachedNodes:    3,
		maxCachedNodes:    10,
		defaultTTL:        24 * time.Hour,
	}
}

// SetSelfAsStorageNode configures this node as a storage node for announcements.
func (sd *StorageNodeDiscovery) SetSelfAsStorageNode(publicKey [32]byte, address string, port uint16, capacity uint32) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.isStorageNode = true
	sd.selfPublicKey = publicKey
	sd.selfAddr = address
	sd.selfPort = port
	sd.selfCapacity = capacity
}

// OnNodeDiscovered sets the callback invoked when a new storage node is discovered.
func (sd *StorageNodeDiscovery) OnNodeDiscovered(callback func(announcement *StorageNodeAnnouncement)) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.onNodeDiscovered = callback
}

// CreateSelfAnnouncement creates an announcement for this node if it's a storage node.
func (sd *StorageNodeDiscovery) CreateSelfAnnouncement(load uint8) *StorageNodeAnnouncement {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if !sd.isStorageNode {
		return nil
	}

	return &StorageNodeAnnouncement{
		PublicKey: sd.selfPublicKey,
		Address:   sd.selfAddr,
		Port:      sd.selfPort,
		Capacity:  sd.selfCapacity,
		Load:      load,
		Timestamp: time.Now(),
		TTL:       sd.defaultTTL,
	}
}

// StoreAnnouncement stores a received storage node announcement.
func (sd *StorageNodeDiscovery) StoreAnnouncement(announcement *StorageNodeAnnouncement) bool {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Don't store our own announcement
	if sd.isStorageNode && announcement.PublicKey == sd.selfPublicKey {
		return false
	}

	// Check if this is a new node
	_, exists := sd.announcements[announcement.PublicKey]
	sd.announcements[announcement.PublicKey] = announcement

	// Invoke callback for new nodes
	if !exists && sd.onNodeDiscovered != nil {
		go sd.onNodeDiscovered(announcement)
	}

	logrus.WithFields(logrus.Fields{
		"function":           "StoreAnnouncement",
		"public_key_preview": formatKeyPreview(announcement.PublicKey[:]),
		"address":            announcement.Address,
		"is_new":             !exists,
	}).Debug("Stored storage node announcement")

	return !exists
}

// GetActiveNodes returns all non-expired storage node announcements.
func (sd *StorageNodeDiscovery) GetActiveNodes() []*StorageNodeAnnouncement {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	var active []*StorageNodeAnnouncement
	for _, ann := range sd.announcements {
		if !ann.IsExpired() {
			active = append(active, ann)
		}
	}
	return active
}

// GetActiveNodesByLoad returns active nodes filtered by maximum load percentage.
func (sd *StorageNodeDiscovery) GetActiveNodesByLoad(maxLoad uint8) []*StorageNodeAnnouncement {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	var filtered []*StorageNodeAnnouncement
	for _, ann := range sd.announcements {
		if !ann.IsExpired() && ann.Load <= maxLoad {
			filtered = append(filtered, ann)
		}
	}
	return filtered
}

// NeedsDiscovery returns true if we have fewer than minCachedNodes active nodes.
func (sd *StorageNodeDiscovery) NeedsDiscovery() bool {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	activeCount := 0
	for _, ann := range sd.announcements {
		if !ann.IsExpired() {
			activeCount++
		}
	}
	return activeCount < sd.minCachedNodes
}

// CleanExpired removes expired announcements from the cache.
func (sd *StorageNodeDiscovery) CleanExpired() int {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	removed := 0
	for pk, ann := range sd.announcements {
		if ann.IsExpired() {
			delete(sd.announcements, pk)
			removed++
		}
	}
	return removed
}

// Count returns the number of cached announcements.
func (sd *StorageNodeDiscovery) Count() int {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return len(sd.announcements)
}

// SerializeAnnouncement serializes a storage node announcement to bytes.
func SerializeAnnouncement(ann *StorageNodeAnnouncement) ([]byte, error) {
	return json.Marshal(ann)
}

// DeserializeAnnouncement deserializes a storage node announcement from bytes.
func DeserializeAnnouncement(data []byte) (*StorageNodeAnnouncement, error) {
	var ann StorageNodeAnnouncement
	if err := json.Unmarshal(data, &ann); err != nil {
		return nil, err
	}
	return &ann, nil
}

// StorageNodeKeyPrefix is the well-known prefix used for DHT storage node discovery.
// It is combined with the node's public key for consistent hashing.
var StorageNodeKeyPrefix = [8]byte{'S', 'T', 'O', 'R', 'A', 'G', 'E', 0x01}

// GenerateStorageNodeKey generates a DHT lookup key for storage node discovery.
func GenerateStorageNodeKey(publicKey [32]byte) [32]byte {
	var key [32]byte
	copy(key[:8], StorageNodeKeyPrefix[:])
	copy(key[8:], publicKey[:24])
	return key
}

// GenerateDiscoveryQueryKey generates a key for querying storage nodes in a region.
// The region is determined by taking the first few bytes of the key space.
func GenerateDiscoveryQueryKey(regionPrefix []byte) [32]byte {
	var key [32]byte
	copy(key[:8], StorageNodeKeyPrefix[:])
	if len(regionPrefix) > 0 {
		maxCopy := 24
		if len(regionPrefix) < maxCopy {
			maxCopy = len(regionPrefix)
		}
		copy(key[8:8+maxCopy], regionPrefix[:maxCopy])
	}
	return key
}

// StorageNodeBinaryAnnouncement provides compact binary serialization for DHT packets.
type StorageNodeBinaryAnnouncement struct {
	PublicKey [32]byte
	Port      uint16
	Capacity  uint32
	Load      uint8
	Timestamp int64 // Unix timestamp
	TTL       int64 // Seconds
	AddrLen   uint8
	Address   []byte // Variable length address string
}

// SerializeBinary serializes the announcement to a compact binary format.
func (a *StorageNodeAnnouncement) SerializeBinary() []byte {
	buf := new(bytes.Buffer)

	buf.Write(a.PublicKey[:])
	binary.Write(buf, binary.BigEndian, a.Port)
	binary.Write(buf, binary.BigEndian, a.Capacity)
	buf.WriteByte(a.Load)
	binary.Write(buf, binary.BigEndian, a.Timestamp.Unix())
	binary.Write(buf, binary.BigEndian, int64(a.TTL.Seconds()))

	addrBytes := []byte(a.Address)
	buf.WriteByte(byte(len(addrBytes)))
	buf.Write(addrBytes)

	return buf.Bytes()
}

// DeserializeAnnouncementBinary deserializes an announcement from binary format.
func DeserializeAnnouncementBinary(data []byte) (*StorageNodeAnnouncement, error) {
	if len(data) < 55 { // Minimum size: 32 + 2 + 4 + 1 + 8 + 8 + 1 = 56, but addr can be 0
		return nil, ErrInvalidAnnouncementData
	}

	ann := &StorageNodeAnnouncement{}

	offset := 0
	copy(ann.PublicKey[:], data[offset:offset+32])
	offset += 32

	ann.Port = binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	ann.Capacity = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	ann.Load = data[offset]
	offset++

	timestamp := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	ann.Timestamp = time.Unix(timestamp, 0)
	offset += 8

	ttlSeconds := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	ann.TTL = time.Duration(ttlSeconds) * time.Second
	offset += 8

	addrLen := int(data[offset])
	offset++

	if offset+addrLen > len(data) {
		return nil, ErrInvalidAnnouncementData
	}
	ann.Address = string(data[offset : offset+addrLen])

	return ann, nil
}

// formatKeyPreview returns a short hex preview of a key for logging.
func formatKeyPreview(key []byte) string {
	if len(key) < 8 {
		return ""
	}
	return string(key[:8])
}
