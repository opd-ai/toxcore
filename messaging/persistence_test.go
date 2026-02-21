package messaging

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// mockMessageStore is a test implementation of MessageStore.
type mockMessageStore struct {
	data      []byte
	saveErr   error
	loadErr   error
	saveCount int
	loadCount int
}

func (s *mockMessageStore) Save(data []byte) error {
	s.saveCount++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.data = make([]byte, len(data))
	copy(s.data, data)
	return nil
}

func (s *mockMessageStore) Load() ([]byte, error) {
	s.loadCount++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.data, nil
}

func TestMessageMarshalJSON(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	lastAttempt := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)

	msg := &Message{
		ID:          42,
		FriendID:    7,
		Type:        MessageTypeNormal,
		Text:        "Hello, world!",
		Timestamp:   timestamp,
		State:       MessageStateSent,
		Retries:     2,
		LastAttempt: lastAttempt,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Verify JSON structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded["id"].(float64) != 42 {
		t.Errorf("Expected ID 42, got %v", decoded["id"])
	}
	if decoded["friend_id"].(float64) != 7 {
		t.Errorf("Expected FriendID 7, got %v", decoded["friend_id"])
	}
	if decoded["text"].(string) != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got %v", decoded["text"])
	}
}

func TestMessageUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": 42,
		"friend_id": 7,
		"type": 0,
		"text": "Test message",
		"timestamp": "2024-01-15T10:30:00Z",
		"state": 2,
		"retries": 1,
		"last_attempt": "2024-01-15T10:31:00Z"
	}`

	var msg Message
	if err := json.Unmarshal([]byte(jsonData), &msg); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if msg.ID != 42 {
		t.Errorf("Expected ID 42, got %d", msg.ID)
	}
	if msg.FriendID != 7 {
		t.Errorf("Expected FriendID 7, got %d", msg.FriendID)
	}
	if msg.Text != "Test message" {
		t.Errorf("Expected text 'Test message', got %s", msg.Text)
	}
	if msg.State != MessageStateSent {
		t.Errorf("Expected state Sent, got %d", msg.State)
	}
	if msg.Retries != 1 {
		t.Errorf("Expected retries 1, got %d", msg.Retries)
	}
}

func TestMessageRoundTrip(t *testing.T) {
	original := &Message{
		ID:          123,
		FriendID:    456,
		Type:        MessageTypeAction,
		Text:        "waves hello",
		Timestamp:   time.Now().UTC().Truncate(time.Second),
		State:       MessageStateDelivered,
		Retries:     3,
		LastAttempt: time.Now().UTC().Truncate(time.Second),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored Message
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", restored.ID, original.ID)
	}
	if restored.FriendID != original.FriendID {
		t.Errorf("FriendID mismatch: got %d, want %d", restored.FriendID, original.FriendID)
	}
	if restored.Type != original.Type {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, original.Type)
	}
	if restored.Text != original.Text {
		t.Errorf("Text mismatch: got %s, want %s", restored.Text, original.Text)
	}
	if !restored.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", restored.Timestamp, original.Timestamp)
	}
	if restored.State != original.State {
		t.Errorf("State mismatch: got %d, want %d", restored.State, original.State)
	}
}

func TestSetStore(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	store := &mockMessageStore{}
	mm.SetStore(store)

	mm.mu.Lock()
	if mm.store != store {
		t.Error("Store was not set correctly")
	}
	mm.mu.Unlock()
}

func TestSaveMessagesNoStore(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	err := mm.SaveMessages()
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Errorf("Expected ErrStoreNotConfigured, got %v", err)
	}
}

func TestLoadMessagesNoStore(t *testing.T) {
	mm := NewMessageManager()
	defer mm.Close()

	err := mm.LoadMessages()
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Errorf("Expected ErrStoreNotConfigured, got %v", err)
	}
}

func TestSaveAndLoadMessages(t *testing.T) {
	store := &mockMessageStore{}

	// Create manager and add messages
	mm1 := NewMessageManager()
	mm1.SetStore(store)

	// Manually add messages (bypassing send logic for test simplicity)
	mm1.mu.Lock()
	mm1.messages[1] = &Message{
		ID:        1,
		FriendID:  100,
		Type:      MessageTypeNormal,
		Text:      "First message",
		Timestamp: time.Now().UTC(),
		State:     MessageStateDelivered,
	}
	mm1.messages[2] = &Message{
		ID:        2,
		FriendID:  100,
		Type:      MessageTypeNormal,
		Text:      "Second message",
		Timestamp: time.Now().UTC(),
		State:     MessageStatePending,
		Retries:   1,
	}
	mm1.nextID = 3
	mm1.mu.Unlock()

	// Save messages
	if err := mm1.SaveMessages(); err != nil {
		t.Fatalf("SaveMessages failed: %v", err)
	}
	mm1.Close()

	if store.saveCount != 1 {
		t.Errorf("Expected 1 save call, got %d", store.saveCount)
	}

	// Create new manager and load messages
	mm2 := NewMessageManager()
	defer mm2.Close()
	mm2.SetStore(store)

	if err := mm2.LoadMessages(); err != nil {
		t.Fatalf("LoadMessages failed: %v", err)
	}

	if store.loadCount != 1 {
		t.Errorf("Expected 1 load call, got %d", store.loadCount)
	}

	// Verify messages were restored
	mm2.mu.Lock()
	if len(mm2.messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(mm2.messages))
	}
	if mm2.nextID != 3 {
		t.Errorf("Expected nextID 3, got %d", mm2.nextID)
	}
	mm2.mu.Unlock()

	// Verify pending message was re-queued
	msg, err := mm2.GetMessage(2)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg.State != MessageStatePending {
		t.Errorf("Expected pending message to be restored as pending, got %d", msg.State)
	}
}

func TestLoadMessagesRestoresPendingQueue(t *testing.T) {
	store := &mockMessageStore{}

	// Pre-populate store with messages in various states
	snapshot := managerSnapshot{
		Messages: []*Message{
			{ID: 1, State: MessageStatePending, Retries: 0},
			{ID: 2, State: MessageStateSending, Retries: 1},
			{ID: 3, State: MessageStateSent, Retries: 0},
			{ID: 4, State: MessageStateDelivered, Retries: 0},
			{ID: 5, State: MessageStateFailed, Retries: 1}, // Should be restored (retries < max)
			{ID: 6, State: MessageStateFailed, Retries: 3}, // Should NOT be restored (retries >= max)
		},
		NextID: 7,
	}

	data, _ := json.Marshal(snapshot)
	store.data = data

	mm := NewMessageManager()
	defer mm.Close()
	mm.SetStore(store)

	if err := mm.LoadMessages(); err != nil {
		t.Fatalf("LoadMessages failed: %v", err)
	}

	// Check pending queue - should contain IDs 1, 2, and 5
	mm.mu.Lock()
	pendingIDs := make(map[uint32]bool)
	for _, msg := range mm.pendingQueue {
		pendingIDs[msg.ID] = true
	}
	mm.mu.Unlock()

	expectedPending := []uint32{1, 2, 5}
	for _, id := range expectedPending {
		if !pendingIDs[id] {
			t.Errorf("Expected message %d in pending queue", id)
		}
	}

	notExpectedPending := []uint32{3, 4, 6}
	for _, id := range notExpectedPending {
		if pendingIDs[id] {
			t.Errorf("Did not expect message %d in pending queue", id)
		}
	}
}

func TestLoadMessagesEmptyStore(t *testing.T) {
	store := &mockMessageStore{} // Empty data

	mm := NewMessageManager()
	defer mm.Close()
	mm.SetStore(store)

	err := mm.LoadMessages()
	if err != nil {
		t.Errorf("LoadMessages should succeed with empty store, got %v", err)
	}

	mm.mu.Lock()
	if len(mm.messages) != 0 {
		t.Errorf("Expected 0 messages for empty store, got %d", len(mm.messages))
	}
	mm.mu.Unlock()
}

func TestSaveMessagesStoreError(t *testing.T) {
	store := &mockMessageStore{
		saveErr: errors.New("disk full"),
	}

	mm := NewMessageManager()
	defer mm.Close()
	mm.SetStore(store)

	mm.mu.Lock()
	mm.messages[1] = &Message{ID: 1, Text: "test"}
	mm.mu.Unlock()

	err := mm.SaveMessages()
	if err == nil {
		t.Error("Expected error from SaveMessages")
	}
	if !errors.Is(err, store.saveErr) {
		t.Errorf("Error should wrap store error")
	}
}

func TestLoadMessagesStoreError(t *testing.T) {
	store := &mockMessageStore{
		loadErr: errors.New("file not found"),
	}

	mm := NewMessageManager()
	defer mm.Close()
	mm.SetStore(store)

	err := mm.LoadMessages()
	if err == nil {
		t.Error("Expected error from LoadMessages")
	}
	if !errors.Is(err, ErrLoadFailed) {
		t.Errorf("Expected ErrLoadFailed, got %v", err)
	}
}

func TestLoadMessagesInvalidJSON(t *testing.T) {
	store := &mockMessageStore{
		data: []byte("not valid json"),
	}

	mm := NewMessageManager()
	defer mm.Close()
	mm.SetStore(store)

	err := mm.LoadMessages()
	if err == nil {
		t.Error("Expected error from LoadMessages with invalid JSON")
	}
	if !errors.Is(err, ErrLoadFailed) {
		t.Errorf("Expected ErrLoadFailed, got %v", err)
	}
}

func TestGetID(t *testing.T) {
	msg := &Message{ID: 42}
	if msg.GetID() != 42 {
		t.Errorf("Expected ID 42, got %d", msg.GetID())
	}
}

func TestGetFriendID(t *testing.T) {
	msg := &Message{FriendID: 123}
	if msg.GetFriendID() != 123 {
		t.Errorf("Expected FriendID 123, got %d", msg.GetFriendID())
	}
}
