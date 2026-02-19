package messaging

import (
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// MessageError represents a messaging error for testing.
type MessageError struct {
	msg string
}

// NewMessageError creates a new MessageError.
func NewMessageError(msg string) *MessageError {
	return &MessageError{msg: msg}
}

func (e *MessageError) Error() string {
	return e.msg
}

// ErrFriendNotFound is a test sentinel error for missing friends.
var ErrFriendNotFound = NewMessageError("friend not found")

// mockKeyProvider implements KeyProvider for testing.
type mockKeyProvider struct {
	friendPublicKeys map[uint32][32]byte
	selfPrivateKey   [32]byte
	selfPublicKey    [32]byte
}

func newMockKeyProvider() *mockKeyProvider {
	keyPair, _ := crypto.GenerateKeyPair()
	return &mockKeyProvider{
		friendPublicKeys: make(map[uint32][32]byte),
		selfPrivateKey:   keyPair.Private,
		selfPublicKey:    keyPair.Public,
	}
}

func (m *mockKeyProvider) GetFriendPublicKey(friendID uint32) ([32]byte, error) {
	key, exists := m.friendPublicKeys[friendID]
	if !exists {
		return [32]byte{}, ErrFriendNotFound
	}
	return key, nil
}

func (m *mockKeyProvider) GetSelfPrivateKey() [32]byte {
	return m.selfPrivateKey
}

// mockTransport implements MessageTransport for testing.
type mockTransport struct {
	sentMessages []*Message
	shouldFail   bool
}

func (m *mockTransport) SendMessagePacket(friendID uint32, message *Message) error {
	if m.shouldFail {
		return NewMessageError("transport failure")
	}
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

// mockTimeProvider provides deterministic time for testing.
type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

func (m *mockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

func (m *mockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}
