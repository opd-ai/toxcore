package toxcore

// toxcore_conference.go contains conference (group chat) functionality for the Tox instance.
// This file is part of the toxcore package refactoring to improve maintainability.

import (
	"errors"
	"fmt"

	"github.com/opd-ai/toxcore/group"
	"github.com/sirupsen/logrus"
)

// ConferenceNew creates a new conference (group chat).
//
//export ToxConferenceNew
func (t *Tox) ConferenceNew() (uint32, error) {
	t.conferencesMu.Lock()
	defer t.conferencesMu.Unlock()

	// Generate unique conference ID
	conferenceID := t.nextConferenceID
	t.nextConferenceID++

	// Create a new group chat for the conference
	// Use CreateWithKeyPair to enable encryption for group messages
	chat, err := group.CreateWithKeyPair("Conference", group.ChatTypeText, group.PrivacyPublic, t.udpTransport, t.dht, t.keyPair)
	if err != nil {
		return 0, fmt.Errorf("failed to create conference: %w", err)
	}

	// Override the ID with our conference ID
	chat.ID = conferenceID

	// Store the conference
	t.conferences[conferenceID] = chat

	return conferenceID, nil
}

// ConferenceInvite invites a friend to a conference.
//
//export ToxConferenceInvite
func (t *Tox) ConferenceInvite(friendID, conferenceID uint32) error {
	// Validate friend exists
	if !t.friends.Exists(friendID) {
		return errors.New("friend not found")
	}

	// Validate conference exists
	t.conferencesMu.RLock()
	conference, exists := t.conferences[conferenceID]
	t.conferencesMu.RUnlock()

	if !exists {
		return errors.New("conference not found")
	}

	// Basic permission check - for now allow all invitations
	// In a full implementation, this would check if the user has invite permissions

	// Generate conference invitation data
	inviteData := fmt.Sprintf("CONF_INVITE:%d:%s", conferenceID, conference.Name)

	// Send invitation through friend messaging system
	_, err := t.FriendSendMessage(friendID, inviteData, MessageTypeNormal)
	if err != nil {
		return fmt.Errorf("failed to send conference invitation: %w", err)
	}

	return nil
}

// ConferenceSendMessage sends a message to a conference.
//
//export ToxConferenceSendMessage
func (t *Tox) ConferenceSendMessage(conferenceID uint32, message string, messageType MessageType) error {
	if err := t.validateConferenceMessage(message); err != nil {
		return err
	}

	conference, err := t.validateConferenceAccess(conferenceID)
	if err != nil {
		return err
	}

	messageData := t.createConferenceMessagePacket(conferenceID, message, messageType)

	return t.broadcastConferenceMessage(conference, messageData)
}

// validateConferenceMessage checks if the conference message input is valid.
func (t *Tox) validateConferenceMessage(message string) error {
	if len(message) == 0 {
		return errors.New("message cannot be empty")
	}

	// Validate message length (Tox message limit)
	if len(message) > 1372 {
		return errors.New("message too long")
	}

	return nil
}

// ValidateConferenceAccess verifies conference exists and user membership.
// Returns the conference Chat object if access is valid, or an error otherwise.
// This method is exported for use by the C API bindings.
func (t *Tox) ValidateConferenceAccess(conferenceID uint32) (*group.Chat, error) {
	return t.validateConferenceAccess(conferenceID)
}

// validateConferenceAccess verifies conference exists and user membership.
func (t *Tox) validateConferenceAccess(conferenceID uint32) (*group.Chat, error) {
	// Validate conference exists
	t.conferencesMu.RLock()
	conference, exists := t.conferences[conferenceID]
	t.conferencesMu.RUnlock()

	if !exists {
		return nil, errors.New("conference not found")
	}

	// Validate we are a member of the conference
	if conference.SelfPeerID == 0 && len(conference.Peers) == 0 {
		return nil, errors.New("not a member of this conference")
	}

	return conference, nil
}

// createConferenceMessagePacket formats the message for conference transmission.
func (t *Tox) createConferenceMessagePacket(conferenceID uint32, message string, messageType MessageType) string {
	// Create conference message packet
	// For now, using a simple packet format without encryption
	return fmt.Sprintf("CONF_MSG:%d:%d:%s", conferenceID, messageType, message)
}

// broadcastConferenceMessage sends the message to all conference peers.
func (t *Tox) broadcastConferenceMessage(conference *group.Chat, messageData string) error {
	broadcastCount := t.sendToConferencePeers(conference, messageData)

	if broadcastCount == 0 && len(conference.Peers) > 1 {
		return errors.New("failed to broadcast to any conference peers")
	}
	return nil
}

// sendToConferencePeers sends a message to all remote conference peers and returns the success count.
func (t *Tox) sendToConferencePeers(conference *group.Chat, messageData string) int {
	count := 0
	for peerID, peer := range conference.Peers {
		if peerID == conference.SelfPeerID {
			continue
		}
		friendID, exists := t.getFriendIDByPublicKey(peer.PublicKey)
		if !exists {
			continue
		}
		if err := t.SendFriendMessage(friendID, messageData, MessageTypeNormal); err == nil {
			count++
		}
	}
	return count
}

// ConferenceDelete leaves and deletes a conference (group chat).
// This removes the local copy of the conference after broadcasting a leave message.
//
//export ToxConferenceDelete
func (t *Tox) ConferenceDelete(conferenceID uint32) error {
	t.conferencesMu.Lock()
	conference, exists := t.conferences[conferenceID]
	if !exists {
		t.conferencesMu.Unlock()
		return errors.New("conference not found")
	}
	// Remove from map while holding lock
	delete(t.conferences, conferenceID)
	t.conferencesMu.Unlock()

	// Call Leave on the group.Chat to broadcast departure and clean up
	if err := conference.Leave(""); err != nil {
		// Log but don't fail - conference already removed locally
		logrus.WithFields(logrus.Fields{
			"function":      "ConferenceDelete",
			"conference_id": conferenceID,
			"error":         err.Error(),
		}).Warn("Failed to broadcast leave message")
	}

	return nil
}
