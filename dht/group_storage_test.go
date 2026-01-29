package dht

import (
	"testing"
	"time"
)

func TestGroupStorage(t *testing.T) {
	storage := NewGroupStorage()
	
	// Create a test announcement
	announcement := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "Test Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       1 * time.Hour,
	}
	
	// Store the announcement
	storage.StoreAnnouncement(announcement)
	
	// Retrieve it
	retrieved, exists := storage.GetAnnouncement(12345)
	if !exists {
		t.Fatal("Expected announcement to exist after storing")
	}
	
	if retrieved.GroupID != announcement.GroupID {
		t.Errorf("Expected GroupID %d, got %d", announcement.GroupID, retrieved.GroupID)
	}
	
	if retrieved.Name != announcement.Name {
		t.Errorf("Expected Name %s, got %s", announcement.Name, retrieved.Name)
	}
}

func TestGroupStorageExpiration(t *testing.T) {
	storage := NewGroupStorage()
	
	// Create an expired announcement
	announcement := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "Expired Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now().Add(-2 * time.Hour), // 2 hours ago
		TTL:       1 * time.Hour,                   // 1 hour TTL
	}
	
	storage.StoreAnnouncement(announcement)
	
	// Try to retrieve - should not exist because it's expired
	_, exists := storage.GetAnnouncement(12345)
	if exists {
		t.Error("Expected expired announcement to not be retrievable")
	}
}

func TestGroupStorageCleanExpired(t *testing.T) {
	storage := NewGroupStorage()
	
	// Add valid announcement
	valid := &GroupAnnouncement{
		GroupID:   11111,
		Name:      "Valid Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       1 * time.Hour,
	}
	storage.StoreAnnouncement(valid)
	
	// Add expired announcement
	expired := &GroupAnnouncement{
		GroupID:   22222,
		Name:      "Expired Group",
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now().Add(-2 * time.Hour),
		TTL:       1 * time.Hour,
	}
	storage.StoreAnnouncement(expired)
	
	// Clean expired
	storage.CleanExpired()
	
	// Valid should still exist
	_, exists := storage.GetAnnouncement(11111)
	if !exists {
		t.Error("Expected valid announcement to still exist after cleanup")
	}
	
	// Check internal map to ensure expired was removed
	storage.mu.RLock()
	_, stillThere := storage.announcements[22222]
	storage.mu.RUnlock()
	
	if stillThere {
		t.Error("Expected expired announcement to be removed from internal map")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	original := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "Test Group Name",
		Type:      1,
		Privacy:   0,
		Timestamp: time.Unix(1640000000, 0), // Fixed timestamp for testing
		TTL:       24 * time.Hour,
	}
	
	// Serialize
	data, err := SerializeAnnouncement(original)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	deserialized, err := DeserializeAnnouncement(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	// Compare
	if deserialized.GroupID != original.GroupID {
		t.Errorf("GroupID mismatch: expected %d, got %d", original.GroupID, deserialized.GroupID)
	}
	
	if deserialized.Name != original.Name {
		t.Errorf("Name mismatch: expected %s, got %s", original.Name, deserialized.Name)
	}
	
	if deserialized.Type != original.Type {
		t.Errorf("Type mismatch: expected %d, got %d", original.Type, deserialized.Type)
	}
	
	if deserialized.Privacy != original.Privacy {
		t.Errorf("Privacy mismatch: expected %d, got %d", original.Privacy, deserialized.Privacy)
	}
	
	if deserialized.Timestamp.Unix() != original.Timestamp.Unix() {
		t.Errorf("Timestamp mismatch: expected %d, got %d", original.Timestamp.Unix(), deserialized.Timestamp.Unix())
	}
}

func TestSerializeEmptyName(t *testing.T) {
	original := &GroupAnnouncement{
		GroupID:   12345,
		Name:      "", // Empty name
		Type:      0,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       1 * time.Hour,
	}
	
	data, err := SerializeAnnouncement(original)
	if err != nil {
		t.Fatalf("Failed to serialize empty name: %v", err)
	}
	
	deserialized, err := DeserializeAnnouncement(data)
	if err != nil {
		t.Fatalf("Failed to deserialize empty name: %v", err)
	}
	
	if deserialized.Name != "" {
		t.Errorf("Expected empty name, got %s", deserialized.Name)
	}
}

func TestDeserializeInvalidData(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{1, 2, 3}},
		{"truncated", []byte{0, 0, 0, 1, 0, 0, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}, // Says 10 byte name but missing
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeserializeAnnouncement(tc.data)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tc.name)
			}
		})
	}
}
