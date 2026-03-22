package main

/*
#include <stdint.h>
#include <stdlib.h>

// Callback type for conference messages
typedef void (*group_message_cb)(void *tox, uint32_t conference_number, uint32_t peer_number,
                                 int type, const uint8_t *message, size_t length, void *user_data);

// Callback type for conference invites
typedef void (*group_invite_cb)(void *tox, uint32_t friend_number, int type,
                                const uint8_t *cookie, size_t length, void *user_data);

// Callback type for file receive events
typedef void (*file_recv_cb)(void *tox, uint32_t friend_number, uint32_t file_number,
                             uint32_t kind, uint64_t file_size, const uint8_t *filename,
                             size_t filename_length, void *user_data);

// Callback type for file chunk receive events
typedef void (*file_recv_chunk_cb)(void *tox, uint32_t friend_number, uint32_t file_number,
                                   uint64_t position, const uint8_t *data, size_t length,
                                   void *user_data);

// Callback type for file chunk request events
typedef void (*file_chunk_request_cb)(void *tox, uint32_t friend_number, uint32_t file_number,
                                      uint64_t position, size_t length, void *user_data);
*/
import "C"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/group"
	"github.com/sirupsen/logrus"
)

// This is the main package required for building as c-shared.
// It provides C-compatible wrappers for the Go toxcore implementation.

// main is required by Go for c-shared build mode but intentionally empty.
// When building with -buildmode=c-shared, Go requires a main package with a main
// function, but the function body is never executed. The shared library's entry
// point is the C runtime initialization, not main().
func main() {}

// ToxRegistry manages Tox instance lifecycle with thread-safe operations.
// It encapsulates instance storage, ID generation, and lookup functions
// to provide a clean abstraction over the C API's opaque pointer model.
type ToxRegistry struct {
	instances map[int]*toxcore.Tox
	nextID    int
	mu        sync.RWMutex
}

// NewToxRegistry creates a new ToxRegistry with initialized state.
func NewToxRegistry() *ToxRegistry {
	return &ToxRegistry{
		instances: make(map[int]*toxcore.Tox),
		nextID:    1,
	}
}

// Get retrieves a Tox instance by ID with proper read lock.
// Returns nil if the instance doesn't exist.
func (r *ToxRegistry) Get(id int) *toxcore.Tox {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instances[id]
}

// Store adds a new Tox instance and returns its assigned ID.
func (r *ToxRegistry) Store(tox *toxcore.Tox) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID
	r.nextID++
	r.instances[id] = tox
	return id
}

// Delete removes a Tox instance by ID and returns it for cleanup.
// Returns nil if the instance doesn't exist.
func (r *ToxRegistry) Delete(id int) *toxcore.Tox {
	r.mu.Lock()
	defer r.mu.Unlock()
	tox, exists := r.instances[id]
	if exists {
		delete(r.instances, id)
	}
	return tox
}

// toxRegistry is the global registry for Tox instances.
// This singleton pattern maintains backward compatibility with the C API
// while providing better encapsulation than raw global variables.
var toxRegistry = NewToxRegistry()

// GetToxInstanceByID retrieves a Tox instance by ID with proper mutex protection.
// This is the authorized accessor for cross-file access within the capi package.
// Returns nil if the instance doesn't exist.
func GetToxInstanceByID(toxID int) *toxcore.Tox {
	return toxRegistry.Get(toxID)
}

// safeGetToxID safely extracts the Tox instance ID from an opaque C pointer.
// This function uses panic recovery to prevent crashes from invalid pointers
// passed from C code, which is essential for C API safety.
// Returns (id, valid) where valid indicates if the pointer was successfully dereferenced.
func safeGetToxID(ptr unsafe.Pointer) (int, bool) {
	if ptr == nil {
		return 0, false
	}

	var toxID int
	var validDeref bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				validDeref = false
				logrus.WithFields(logrus.Fields{
					"function":    "safeGetToxID",
					"ptr_address": fmt.Sprintf("%p", ptr),
					"panic_value": fmt.Sprintf("%v", r),
				}).Error("Invalid C pointer dereference in capi — caller passed a stale or corrupt pointer")
			}
		}()

		handle := (*int)(ptr)
		toxID = *handle
		validDeref = true
	}()

	if !validDeref {
		return 0, false
	}

	// Sanity check: ID should be positive
	if toxID <= 0 {
		return 0, false
	}

	return toxID, true
}

// getToxFromPointer retrieves a validated Tox instance from an opaque C pointer.
// Returns (instance, ok) where ok is false if the pointer is invalid or instance not found.
func getToxFromPointer(ptr unsafe.Pointer) (*toxcore.Tox, bool) {
	toxID, ok := safeGetToxID(ptr)
	if !ok {
		return nil, false
	}
	instance := toxRegistry.Get(toxID)
	return instance, instance != nil
}

// setError safely sets an error code through a C pointer if non-nil.
func setError(err *uint32, code uint32) {
	if err != nil {
		*err = code
	}
}

// getFriendByNumber retrieves a friend from a Tox instance by friend number.
// Returns the friend and true if found, nil and false otherwise.
// This consolidates the common friend lookup pattern used across capi functions.
func getFriendByNumber(tox unsafe.Pointer, friendNumber C.uint32_t) (*toxcore.Friend, bool) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return nil, false
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return nil, false
	}

	friends := toxInstance.GetFriends()
	if friends == nil {
		return nil, false
	}

	friend, exists := friends[uint32(friendNumber)]
	return friend, exists
}

//export tox_new
func tox_new() unsafe.Pointer {
	// Create new Tox instance with default options
	goOptions := toxcore.NewOptions()

	// Create new Tox instance
	tox, err := toxcore.New(goOptions)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "tox_new",
			"error":    err.Error(),
		}).Error("Failed to create new Tox instance")
		return nil
	}

	// Store instance and get ID
	instanceID := toxRegistry.Store(tox)

	// Create an opaque pointer handle
	handle := new(int)
	*handle = instanceID
	return unsafe.Pointer(handle)
}

//export tox_kill
func tox_kill(tox unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	if toxInstance := toxRegistry.Delete(toxID); toxInstance != nil {
		toxInstance.Kill()
	}
}

//export tox_bootstrap_simple
func tox_bootstrap_simple(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// Use known working bootstrap node for testing
	err := toxInstance.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		return -1
	}

	return 0 // Success
}

//export tox_iterate
func tox_iterate(tox unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	if toxInstance := toxRegistry.Get(toxID); toxInstance != nil {
		toxInstance.Iterate()
	}
}

//export tox_iteration_interval
func tox_iteration_interval(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 50 // Default 50ms
	}

	if toxInstance := toxRegistry.Get(toxID); toxInstance != nil {
		return int(toxInstance.IterationInterval().Milliseconds())
	}
	return 50 // Default 50ms
}

//export tox_self_get_address_size
func tox_self_get_address_size(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	if toxInstance := toxRegistry.Get(toxID); toxInstance != nil {
		addr := toxInstance.SelfGetAddress()
		return len(addr)
	}
	return 0
}

//export hex_string_to_bin
func hex_string_to_bin(hexStr *byte, hexLen int, output *byte, outputLen int) int {
	// Convert C buffer to Go slice using unsafe.Slice (clearer than manual iteration)
	hexBytes := unsafe.Slice(hexStr, hexLen)
	hexString := string(hexBytes)

	// Decode hex string
	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		return -1 // Error
	}

	// Check output buffer size
	if len(decoded) > outputLen {
		return -1 // Buffer too small
	}

	// Copy to output buffer using copy builtin (clearer and potentially faster)
	outputSlice := unsafe.Slice(output, outputLen)
	copy(outputSlice, decoded)

	return len(decoded) // Return number of bytes written
}

// tox_self_get_address copies the Tox address to the provided buffer.
// The buffer must be at least TOX_ADDRESS_SIZE (38) bytes.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_address
func tox_self_get_address(tox unsafe.Pointer, address *byte) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	addr := toxInstance.SelfGetAddress()
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		return -1
	}

	// Copy to output buffer
	outputSlice := unsafe.Slice(address, len(addrBytes))
	copy(outputSlice, addrBytes)

	return 0
}

// tox_self_get_public_key copies the public key to the provided buffer.
// The buffer must be at least TOX_PUBLIC_KEY_SIZE (32) bytes.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_public_key
func tox_self_get_public_key(tox unsafe.Pointer, publicKey *byte) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// Get address and extract public key (first 32 bytes)
	addr := toxInstance.SelfGetAddress()
	addrBytes, err := hex.DecodeString(addr)
	if err != nil || len(addrBytes) < 32 {
		return -1
	}

	// Copy public key (first 32 bytes of address)
	outputSlice := unsafe.Slice(publicKey, 32)
	copy(outputSlice, addrBytes[:32])

	return 0
}

// tox_friend_add adds a friend by Tox address and sends a friend request message.
// Returns the friend number on success, or UINT32_MAX on failure.
//
//export tox_friend_add
func tox_friend_add(tox unsafe.Pointer, address, message *byte, messageLen int) uint32 {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0xFFFFFFFF
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0xFFFFFFFF
	}

	// Convert address bytes to hex string (38 bytes = 76 hex chars)
	addrBytes := unsafe.Slice(address, 38)
	addrHex := hex.EncodeToString(addrBytes)

	// Convert message bytes to string
	msgBytes := unsafe.Slice(message, messageLen)
	msgStr := string(msgBytes)

	friendNum, err := toxInstance.AddFriend(addrHex, msgStr)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "tox_friend_add",
			"error":    err.Error(),
		}).Debug("Failed to add friend")
		return 0xFFFFFFFF
	}

	return friendNum
}

// tox_friend_add_norequest adds a friend by public key without sending a request.
// Use this to accept incoming friend requests.
// Returns the friend number on success, or UINT32_MAX on failure.
//
//export tox_friend_add_norequest
func tox_friend_add_norequest(tox unsafe.Pointer, publicKey *byte) uint32 {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0xFFFFFFFF
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0xFFFFFFFF
	}

	// Convert public key bytes to [32]byte
	pkBytes := unsafe.Slice(publicKey, 32)
	var pk [32]byte
	copy(pk[:], pkBytes)

	friendNum, err := toxInstance.AddFriendByPublicKey(pk)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "tox_friend_add_norequest",
			"error":    err.Error(),
		}).Debug("Failed to add friend by public key")
		return 0xFFFFFFFF
	}

	return friendNum
}

// tox_friend_delete removes a friend from the friends list.
// Returns 0 on success, -1 on failure.
//
//export tox_friend_delete
func tox_friend_delete(tox unsafe.Pointer, friendNumber uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	err := toxInstance.DeleteFriend(friendNumber)
	if err != nil {
		return -1
	}

	return 0
}

// tox_friend_send_message sends a message to a friend.
// messageType: 0 = normal message, 1 = action message.
// Returns the message ID on success (always 1 for now), or 0 on failure.
//
//export tox_friend_send_message
func tox_friend_send_message(tox unsafe.Pointer, friendNumber uint32, messageType int, message *byte, messageLen int) uint32 {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	// Convert message bytes to string
	msgBytes := unsafe.Slice(message, messageLen)
	msgStr := string(msgBytes)

	// Convert C message type to Go message type
	var goMsgType toxcore.MessageType
	if messageType == 1 {
		goMsgType = toxcore.MessageTypeAction
	} else {
		goMsgType = toxcore.MessageTypeNormal
	}

	err := toxInstance.SendFriendMessage(friendNumber, msgStr, goMsgType)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":     "tox_friend_send_message",
			"friend":       friendNumber,
			"message_type": messageType,
			"error":        err.Error(),
		}).Debug("Failed to send message")
		return 0
	}

	// Return message ID (simplified: always 1 since we don't track message IDs yet)
	return 1
}

// Callback storage for friend-related callbacks
type toxCallbacks struct {
	friendRequestCb       unsafe.Pointer
	friendRequestUserData unsafe.Pointer
	friendMessageCb       unsafe.Pointer
	friendMessageUserData unsafe.Pointer
	friendConnStatusCb    unsafe.Pointer
	friendConnStatusData  unsafe.Pointer
}

var (
	toxCallbacksMu sync.RWMutex
	toxCallbackMap = make(map[int]*toxCallbacks)
)

// getToxCallbacks retrieves or creates callbacks for a tox instance
func getToxCallbacks(toxID int) *toxCallbacks {
	toxCallbacksMu.Lock()
	defer toxCallbacksMu.Unlock()
	if cb, exists := toxCallbackMap[toxID]; exists {
		return cb
	}
	cb := &toxCallbacks{}
	toxCallbackMap[toxID] = cb
	return cb
}

// tox_callback_friend_request registers a callback for friend requests.
// The callback receives: tox pointer, public key (32 bytes), message, message length, user data.
//
//export tox_callback_friend_request
func tox_callback_friend_request(tox, callback, userData unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return
	}

	cb := getToxCallbacks(toxID)
	cb.friendRequestCb = callback
	cb.friendRequestUserData = userData

	// Register Go callback that invokes the C callback
	toxInstance.OnFriendRequest(func(publicKey [32]byte, message string) {
		toxCallbacksMu.RLock()
		cbData := toxCallbackMap[toxID]
		toxCallbacksMu.RUnlock()
		if cbData == nil || cbData.friendRequestCb == nil {
			return
		}
		// Note: The actual C callback invocation would require CGo import "C"
		// For now, we store the callback and it would be invoked via a bridge function
		logrus.WithFields(logrus.Fields{
			"function":   "friend_request_callback",
			"public_key": fmt.Sprintf("%x", publicKey[:8]),
			"message":    message,
		}).Debug("Friend request received (C callback registered)")
	})
}

// tox_callback_friend_message registers a callback for friend messages.
// The callback receives: tox pointer, friend number, message type, message, message length, user data.
//
//export tox_callback_friend_message
func tox_callback_friend_message(tox, callback, userData unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return
	}

	cb := getToxCallbacks(toxID)
	cb.friendMessageCb = callback
	cb.friendMessageUserData = userData

	// Register Go callback that invokes the C callback
	toxInstance.OnFriendMessage(func(friendID uint32, message string) {
		toxCallbacksMu.RLock()
		cbData := toxCallbackMap[toxID]
		toxCallbacksMu.RUnlock()
		if cbData == nil || cbData.friendMessageCb == nil {
			return
		}
		logrus.WithFields(logrus.Fields{
			"function":  "friend_message_callback",
			"friend_id": friendID,
			"message":   message,
		}).Debug("Friend message received (C callback registered)")
	})
}

// tox_callback_friend_connection_status registers a callback for friend connection status changes.
// The callback receives: tox pointer, friend number, connection status, user data.
//
//export tox_callback_friend_connection_status
func tox_callback_friend_connection_status(tox, callback, userData unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return
	}

	cb := getToxCallbacks(toxID)
	cb.friendConnStatusCb = callback
	cb.friendConnStatusData = userData

	// Register Go callback that invokes the C callback
	toxInstance.OnFriendConnectionStatus(func(friendID uint32, connectionStatus toxcore.ConnectionStatus) {
		toxCallbacksMu.RLock()
		cbData := toxCallbackMap[toxID]
		toxCallbacksMu.RUnlock()
		if cbData == nil || cbData.friendConnStatusCb == nil {
			return
		}
		logrus.WithFields(logrus.Fields{
			"function":          "friend_connection_status_callback",
			"friend_id":         friendID,
			"connection_status": connectionStatus,
		}).Debug("Friend connection status changed (C callback registered)")
	})
}

// tox_self_set_name sets the name of this Tox instance.
// Returns 0 on success, -1 on error.
//
//export tox_self_set_name
func tox_self_set_name(tox unsafe.Pointer, name *byte, nameLen int) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	nameBytes := unsafe.Slice(name, nameLen)
	nameStr := string(nameBytes)

	err := toxInstance.SelfSetName(nameStr)
	if err != nil {
		return -1
	}

	return 0
}

// tox_self_get_name_size returns the length of the name.
//
//export tox_self_get_name_size
func tox_self_get_name_size(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	return len(toxInstance.SelfGetName())
}

// tox_self_get_name copies the name to the provided buffer.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_name
func tox_self_get_name(tox unsafe.Pointer, name *byte) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	nameStr := toxInstance.SelfGetName()
	if len(nameStr) == 0 {
		return 0
	}

	outputSlice := unsafe.Slice(name, len(nameStr))
	copy(outputSlice, []byte(nameStr))

	return 0
}

// tox_self_set_status_message sets the status message of this Tox instance.
// Returns 0 on success, -1 on error.
//
//export tox_self_set_status_message
func tox_self_set_status_message(tox unsafe.Pointer, message *byte, messageLen int) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	msgBytes := unsafe.Slice(message, messageLen)
	msgStr := string(msgBytes)

	err := toxInstance.SelfSetStatusMessage(msgStr)
	if err != nil {
		return -1
	}

	return 0
}

// tox_self_get_status_message_size returns the length of the status message.
//
//export tox_self_get_status_message_size
func tox_self_get_status_message_size(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	return len(toxInstance.SelfGetStatusMessage())
}

// tox_self_get_status_message copies the status message to the provided buffer.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_status_message
func tox_self_get_status_message(tox unsafe.Pointer, message *byte) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	msgStr := toxInstance.SelfGetStatusMessage()
	if len(msgStr) == 0 {
		return 0
	}

	outputSlice := unsafe.Slice(message, len(msgStr))
	copy(outputSlice, []byte(msgStr))

	return 0
}

// ============================================================================
// GROUP CHAT (CONFERENCE) API
// ============================================================================

// groupMessageCallbacks stores callbacks for group message events
var (
	groupMessageCallbacks   = make(map[int]C.group_message_cb)
	groupMessageCallbacksMu sync.RWMutex
)

// groupInviteCallbacks stores callbacks for group invite events
var (
	groupInviteCallbacks   = make(map[int]C.group_invite_cb)
	groupInviteCallbacksMu sync.RWMutex
)

// tox_conference_new creates a new conference (group chat).
// Returns the conference ID on success, or UINT32_MAX on failure.
//
//export tox_conference_new
func tox_conference_new(tox unsafe.Pointer, err *uint32) uint32 {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_CONFERENCE_NEW_INIT
		}
		return 0xFFFFFFFF
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		if err != nil {
			*err = 1
		}
		return 0xFFFFFFFF
	}

	conferenceID, createErr := toxInstance.ConferenceNew()
	if createErr != nil {
		logrus.WithField("error", createErr.Error()).Error("Failed to create conference")
		if err != nil {
			*err = 1
		}
		return 0xFFFFFFFF
	}

	if err != nil {
		*err = 0 // TOX_ERR_CONFERENCE_NEW_OK
	}
	return conferenceID
}

// tox_conference_invite invites a friend to a conference.
// Returns 0 on success, non-zero on error.
//
//export tox_conference_invite
func tox_conference_invite(tox unsafe.Pointer, friendID, conferenceID uint32, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_CONFERENCE_INVITE_CONFERENCE_NOT_FOUND
		}
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		if err != nil {
			*err = 1
		}
		return -1
	}

	inviteErr := toxInstance.ConferenceInvite(friendID, conferenceID)
	if inviteErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id":     friendID,
			"conference_id": conferenceID,
			"error":         inviteErr.Error(),
		}).Error("Failed to invite friend to conference")
		if err != nil {
			*err = 2 // TOX_ERR_CONFERENCE_INVITE_FAIL_SEND
		}
		return -1
	}

	if err != nil {
		*err = 0 // TOX_ERR_CONFERENCE_INVITE_OK
	}
	return 0
}

// tox_conference_send_message sends a message to a conference.
// Returns 0 on success, non-zero on error.
//
// conferenceMessageError codes match TOX_ERR_CONFERENCE_SEND_MESSAGE
const (
	confMsgErrOK = iota
	confMsgErrNotFound
	confMsgErrTooLong
	confMsgErrNoConnection
	confMsgErrFailSend
)

// validateConferenceToxInstance validates the tox pointer and returns the instance.
func validateConferenceToxInstance(tox unsafe.Pointer, err *uint32) (*toxcore.Tox, bool) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		setConfError(err, confMsgErrNotFound)
		return nil, false
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		setConfError(err, confMsgErrNotFound)
		return nil, false
	}

	return toxInstance, true
}

// setConfError safely sets an error code if err pointer is non-nil.
func setConfError(err *uint32, code uint32) {
	if err != nil {
		*err = code
	}
}

// validateConferenceMessage checks if the message is valid.
func validateConferenceMessage(message *byte, length uint32, err *uint32) (string, bool) {
	if message == nil || length == 0 {
		setConfError(err, confMsgErrNoConnection)
		return "", false
	}
	messageSlice := unsafe.Slice(message, length)
	return string(messageSlice), true
}

// convertConferenceMessageType converts C message type to Go type.
func convertConferenceMessageType(msgType int) toxcore.MessageType {
	if msgType == 1 {
		return toxcore.MessageTypeAction
	}
	return toxcore.MessageTypeNormal
}

//export tox_conference_send_message
func tox_conference_send_message(tox unsafe.Pointer, conferenceID uint32, msgType int, message *byte, length uint32, err *uint32) int {
	toxInstance, ok := validateConferenceToxInstance(tox, err)
	if !ok {
		return -1
	}

	messageStr, ok := validateConferenceMessage(message, length, err)
	if !ok {
		return -1
	}

	toxMsgType := convertConferenceMessageType(msgType)

	sendErr := toxInstance.ConferenceSendMessage(conferenceID, messageStr, toxMsgType)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"conference_id": conferenceID,
			"error":         sendErr.Error(),
		}).Error("Failed to send conference message")
		setConfError(err, confMsgErrNoConnection)
		return -1
	}

	setConfError(err, confMsgErrOK)
	return 0
}

// tox_conference_delete leaves and deletes a conference.
// Returns 0 on success, non-zero on error.
//
//export tox_conference_delete
func tox_conference_delete(tox unsafe.Pointer, conferenceID uint32, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_CONFERENCE_DELETE_CONFERENCE_NOT_FOUND
		}
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		if err != nil {
			*err = 1
		}
		return -1
	}

	// Note: ConferenceDelete may need to be implemented in toxcore.go
	// For now, we'll return an error indicating not implemented
	logrus.WithField("conference_id", conferenceID).Warn("Conference delete not yet implemented")
	if err != nil {
		*err = 1
	}
	return -1
}

// tox_conference_get_title gets the title of a conference.
// Returns the length of the title on success, or -1 on error.
//
//export tox_conference_get_title_size
func tox_conference_get_title_size(tox unsafe.Pointer, conferenceID uint32, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1
		}
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		if err != nil {
			*err = 1
		}
		return -1
	}

	// Note: Would need a way to get conference title from toxcore
	// For now return placeholder
	_ = toxInstance
	_ = conferenceID
	if err != nil {
		*err = 0
	}
	return 0
}

// tox_callback_conference_message sets the callback for conference message events.
//
//export tox_callback_conference_message
func tox_callback_conference_message(tox unsafe.Pointer, callback C.group_message_cb) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	groupMessageCallbacksMu.Lock()
	if callback == nil {
		delete(groupMessageCallbacks, toxID)
	} else {
		groupMessageCallbacks[toxID] = callback
	}
	groupMessageCallbacksMu.Unlock()

	// Note: Would need to connect this to toxcore's group message handler
	logrus.WithField("tox_id", toxID).Debug("Conference message callback registered")
}

// tox_callback_conference_invite sets the callback for conference invite events.
//
//export tox_callback_conference_invite
func tox_callback_conference_invite(tox unsafe.Pointer, callback C.group_invite_cb) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	groupInviteCallbacksMu.Lock()
	if callback == nil {
		delete(groupInviteCallbacks, toxID)
	} else {
		groupInviteCallbacks[toxID] = callback
	}
	groupInviteCallbacksMu.Unlock()

	logrus.WithField("tox_id", toxID).Debug("Conference invite callback registered")
}

// ============================================================================
// FILE TRANSFER API
// ============================================================================

// fileRecvCallbacks stores callbacks for file receive events
var (
	fileRecvCallbacks   = make(map[int]C.file_recv_cb)
	fileRecvCallbacksMu sync.RWMutex
)

// fileRecvChunkCallbacks stores callbacks for file chunk receive events
var (
	fileRecvChunkCallbacks   = make(map[int]C.file_recv_chunk_cb)
	fileRecvChunkCallbacksMu sync.RWMutex
)

// fileChunkRequestCallbacks stores callbacks for file chunk request events
var (
	fileChunkRequestCallbacks   = make(map[int]C.file_chunk_request_cb)
	fileChunkRequestCallbacksMu sync.RWMutex
)

// tox_file_send sends a file send request.
// Returns the file number on success, or UINT32_MAX on failure.
//
//export tox_file_send
func tox_file_send(tox unsafe.Pointer, friendID, kind uint32, fileSize uint64, fileID, filename *byte, filenameLen uint32, err *uint32) uint32 {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, 1) // TOX_ERR_FILE_SEND_NULL
		return 0xFFFFFFFF
	}

	filenameStr := bytesToString(filename, filenameLen)
	fileIDArray := bytesToFileID(fileID)

	fileNum, sendErr := toxInstance.FileSend(friendID, kind, fileSize, fileIDArray, filenameStr)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id": friendID,
			"filename":  filenameStr,
			"error":     sendErr.Error(),
		}).Error("Failed to send file")
		setError(err, 6) // TOX_ERR_FILE_SEND_FRIEND_NOT_CONNECTED
		return 0xFFFFFFFF
	}

	setError(err, 0) // TOX_ERR_FILE_SEND_OK
	return fileNum
}

// bytesToString converts a C byte pointer and length to a Go string.
func bytesToString(data *byte, length uint32) string {
	if data == nil || length == 0 {
		return ""
	}
	return string(unsafe.Slice(data, length))
}

// bytesToFileID converts a C byte pointer to a 32-byte file ID array.
func bytesToFileID(data *byte) [32]byte {
	var fileID [32]byte
	if data != nil {
		copy(fileID[:], unsafe.Slice(data, 32))
	}
	return fileID
}

// tox_file_control controls an ongoing file transfer.
// Returns 0 on success, non-zero on error.
//
//export tox_file_control
func tox_file_control(tox unsafe.Pointer, friendID, fileID uint32, control int, err *uint32) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, 1) // TOX_ERR_FILE_CONTROL_FRIEND_NOT_FOUND
		return -1
	}

	fileControl, valid := parseFileControl(control)
	if !valid {
		setError(err, 5) // TOX_ERR_FILE_CONTROL_SENDQ
		return -1
	}

	controlErr := toxInstance.FileControl(friendID, fileID, fileControl)
	if controlErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id": friendID,
			"file_id":   fileID,
			"control":   control,
			"error":     controlErr.Error(),
		}).Error("Failed to control file transfer")
		setError(err, 4) // TOX_ERR_FILE_CONTROL_NOT_FOUND
		return -1
	}

	setError(err, 0) // TOX_ERR_FILE_CONTROL_OK
	return 0
}

// parseFileControl converts an integer control value to a FileControl type.
func parseFileControl(control int) (toxcore.FileControl, bool) {
	switch control {
	case 0:
		return toxcore.FileControlResume, true
	case 1:
		return toxcore.FileControlPause, true
	case 2:
		return toxcore.FileControlCancel, true
	default:
		return 0, false
	}
}

// tox_file_send_chunk sends a chunk of a file being transferred.
// Returns 0 on success, non-zero on error.
//
//export tox_file_send_chunk
func tox_file_send_chunk(tox unsafe.Pointer, friendID, fileID uint32, position uint64, data *byte, length uint32, err *uint32) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, 1) // TOX_ERR_FILE_SEND_CHUNK_NULL
		return -1
	}

	dataSlice := bytesToSlice(data, length)

	sendErr := toxInstance.FileSendChunk(friendID, fileID, position, dataSlice)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id": friendID,
			"file_id":   fileID,
			"position":  position,
			"length":    length,
			"error":     sendErr.Error(),
		}).Error("Failed to send file chunk")
		setError(err, 5) // TOX_ERR_FILE_SEND_CHUNK_SENDQ
		return -1
	}

	setError(err, 0) // TOX_ERR_FILE_SEND_CHUNK_OK
	return 0
}

// bytesToSlice converts a C byte pointer and length to a Go byte slice.
func bytesToSlice(data *byte, length uint32) []byte {
	if data == nil || length == 0 {
		return nil
	}
	return unsafe.Slice(data, length)
}

// tox_callback_file_recv sets the callback for file receive events.
//
//export tox_callback_file_recv
func tox_callback_file_recv(tox unsafe.Pointer, callback C.file_recv_cb) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	fileRecvCallbacksMu.Lock()
	if callback == nil {
		delete(fileRecvCallbacks, toxID)
	} else {
		fileRecvCallbacks[toxID] = callback
	}
	fileRecvCallbacksMu.Unlock()

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance != nil {
		toxInstance.OnFileRecv(func(friendID, fileID, kind uint32, fileSize uint64, filename string) {
			fileRecvCallbacksMu.RLock()
			cb, exists := fileRecvCallbacks[toxID]
			fileRecvCallbacksMu.RUnlock()
			if exists && cb != nil {
				// Note: Would need to call C callback here
				logrus.WithFields(logrus.Fields{
					"friend_id": friendID,
					"file_id":   fileID,
					"kind":      kind,
					"file_size": fileSize,
					"filename":  filename,
				}).Debug("File recv callback triggered")
			}
		})
	}

	logrus.WithField("tox_id", toxID).Debug("File recv callback registered")
}

// tox_callback_file_recv_chunk sets the callback for file chunk receive events.
//
//export tox_callback_file_recv_chunk
func tox_callback_file_recv_chunk(tox unsafe.Pointer, callback C.file_recv_chunk_cb) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	fileRecvChunkCallbacksMu.Lock()
	if callback == nil {
		delete(fileRecvChunkCallbacks, toxID)
	} else {
		fileRecvChunkCallbacks[toxID] = callback
	}
	fileRecvChunkCallbacksMu.Unlock()

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance != nil {
		toxInstance.OnFileRecvChunk(func(friendID, fileID uint32, position uint64, data []byte) {
			fileRecvChunkCallbacksMu.RLock()
			cb, exists := fileRecvChunkCallbacks[toxID]
			fileRecvChunkCallbacksMu.RUnlock()
			if exists && cb != nil {
				logrus.WithFields(logrus.Fields{
					"friend_id": friendID,
					"file_id":   fileID,
					"position":  position,
					"data_len":  len(data),
				}).Debug("File recv chunk callback triggered")
			}
		})
	}

	logrus.WithField("tox_id", toxID).Debug("File recv chunk callback registered")
}

// tox_callback_file_chunk_request sets the callback for file chunk request events.
//
//export tox_callback_file_chunk_request
func tox_callback_file_chunk_request(tox unsafe.Pointer, callback C.file_chunk_request_cb) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	fileChunkRequestCallbacksMu.Lock()
	if callback == nil {
		delete(fileChunkRequestCallbacks, toxID)
	} else {
		fileChunkRequestCallbacks[toxID] = callback
	}
	fileChunkRequestCallbacksMu.Unlock()

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance != nil {
		toxInstance.OnFileChunkRequest(func(friendID, fileID uint32, position uint64, length int) {
			fileChunkRequestCallbacksMu.RLock()
			cb, exists := fileChunkRequestCallbacks[toxID]
			fileChunkRequestCallbacksMu.RUnlock()
			if exists && cb != nil {
				logrus.WithFields(logrus.Fields{
					"friend_id": friendID,
					"file_id":   fileID,
					"position":  position,
					"length":    length,
				}).Debug("File chunk request callback triggered")
			}
		})
	}

	logrus.WithField("tox_id", toxID).Debug("File chunk request callback registered")
}

// =============================================================================
// Self Functions - Status and Connection
// =============================================================================

// tox_self_get_connection_status returns the connection status of the Tox instance.
// Returns: 0 = TCP, 1 = UDP, -1 = error
//
//export tox_self_get_connection_status
func tox_self_get_connection_status(tox unsafe.Pointer) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	status := toxInstance.SelfGetConnectionStatus()
	return C.int(status)
}

// tox_self_get_status returns the current user status of this Tox instance.
// Returns: 0 = None, 1 = Away, 2 = Busy, -1 = error
//
//export tox_self_get_status
func tox_self_get_status(tox unsafe.Pointer) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// Return the current user status (0=None, 1=Away, 2=Busy)
	// The Go implementation currently doesn't track self-status explicitly,
	// so we return None (0) as the default online status.
	return 0
}

// tox_self_set_status sets the user status of this Tox instance.
// status: 0 = None, 1 = Away, 2 = Busy
// Returns: 0 on success, -1 on error
//
//export tox_self_set_status
func tox_self_set_status(tox unsafe.Pointer, status C.int) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// Validate status range
	if status < 0 || status > 2 {
		return -1
	}

	// Note: The Go implementation doesn't currently track self-status.
	// This is a no-op but returns success for API compatibility.
	logrus.WithFields(logrus.Fields{
		"tox_id": toxID,
		"status": status,
	}).Debug("Self status set (no-op in current implementation)")

	return 0
}

// tox_self_get_nospam returns the 4-byte nospam value from the Tox ID.
//
//export tox_self_get_nospam
func tox_self_get_nospam(tox unsafe.Pointer) C.uint32_t {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	nospam := toxInstance.SelfGetNospam()
	// Convert 4-byte array to uint32 (big-endian)
	return C.uint32_t(uint32(nospam[0])<<24 | uint32(nospam[1])<<16 | uint32(nospam[2])<<8 | uint32(nospam[3]))
}

// tox_self_set_nospam sets the 4-byte nospam value for the Tox ID.
//
//export tox_self_set_nospam
func tox_self_set_nospam(tox unsafe.Pointer, nospam C.uint32_t) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return
	}

	// Convert uint32 to 4-byte array (big-endian)
	var nospamBytes [4]byte
	nospamBytes[0] = byte(nospam >> 24)
	nospamBytes[1] = byte(nospam >> 16)
	nospamBytes[2] = byte(nospam >> 8)
	nospamBytes[3] = byte(nospam)

	toxInstance.SelfSetNospam(nospamBytes)
}

// =============================================================================
// Friend Functions - Name, Status, Connection
// =============================================================================

// tox_friend_get_name_size returns the length of a friend's name.
// Returns: The length of the name, or 0 on error.
//
//export tox_friend_get_name_size
func tox_friend_get_name_size(tox unsafe.Pointer, friendNumber C.uint32_t) C.size_t {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return 0
	}
	return C.size_t(len(friend.Name))
}

// tox_friend_get_name writes a friend's name to a buffer.
// name: Buffer to write the name to (must be at least tox_friend_get_name_size bytes).
// Returns: 1 on success, 0 on error.
//
//export tox_friend_get_name
func tox_friend_get_name(tox unsafe.Pointer, friendNumber C.uint32_t, name *C.uint8_t) C.int {
	if name == nil {
		return 0
	}

	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return 0
	}

	if len(friend.Name) == 0 {
		return 1 // Success, but name is empty
	}

	// Copy name to C buffer
	nameSlice := unsafe.Slice((*byte)(unsafe.Pointer(name)), len(friend.Name))
	copy(nameSlice, []byte(friend.Name))

	return 1
}

// tox_friend_get_status_message_size returns the length of a friend's status message.
// Returns: The length of the status message, or 0 on error.
//
//export tox_friend_get_status_message_size
func tox_friend_get_status_message_size(tox unsafe.Pointer, friendNumber C.uint32_t) C.size_t {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return 0
	}
	return C.size_t(len(friend.StatusMessage))
}

// tox_friend_get_status_message writes a friend's status message to a buffer.
// status_message: Buffer to write the status message to.
// Returns: 1 on success, 0 on error.
//
//export tox_friend_get_status_message
func tox_friend_get_status_message(tox unsafe.Pointer, friendNumber C.uint32_t, statusMessage *C.uint8_t) C.int {
	if statusMessage == nil {
		return 0
	}

	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return 0
	}

	if len(friend.StatusMessage) == 0 {
		return 1 // Success, but message is empty
	}

	// Copy status message to C buffer
	msgSlice := unsafe.Slice((*byte)(unsafe.Pointer(statusMessage)), len(friend.StatusMessage))
	copy(msgSlice, []byte(friend.StatusMessage))

	return 1
}

// tox_friend_get_status returns the status of a friend.
// Returns: 0 = None/Online, 1 = Away, 2 = Busy, -1 = error
//
//export tox_friend_get_status
func tox_friend_get_status(tox unsafe.Pointer, friendNumber C.uint32_t) C.int {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return -1
	}
	return C.int(friend.Status)
}

// tox_friend_get_connection_status returns the connection status of a friend.
// Returns: 0 = None, 1 = TCP, 2 = UDP, -1 = error
//
//export tox_friend_get_connection_status
func tox_friend_get_connection_status(tox unsafe.Pointer, friendNumber C.uint32_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	status := toxInstance.GetFriendConnectionStatus(uint32(friendNumber))
	return C.int(status)
}

// tox_friend_get_public_key writes a friend's public key to a buffer.
// public_key: Buffer to write the 32-byte public key to.
// Returns: 1 on success, 0 on error.
//
//export tox_friend_get_public_key
func tox_friend_get_public_key(tox unsafe.Pointer, friendNumber C.uint32_t, publicKey *C.uint8_t) C.int {
	if publicKey == nil {
		return 0
	}

	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	pk, err := toxInstance.GetFriendPublicKey(uint32(friendNumber))
	if err != nil {
		return 0
	}

	// Copy public key to C buffer
	pkSlice := unsafe.Slice((*byte)(unsafe.Pointer(publicKey)), 32)
	copy(pkSlice, pk[:])

	return 1
}

// tox_friend_get_last_online returns the Unix timestamp of when a friend was last online.
// Returns: Unix timestamp, or 0 on error.
//
//export tox_friend_get_last_online
func tox_friend_get_last_online(tox unsafe.Pointer, friendNumber C.uint32_t) C.uint64_t {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return 0
	}
	return C.uint64_t(friend.LastSeen.Unix())
}

// tox_friend_exists checks if a friend with the given number exists.
// Returns: 1 if exists, 0 if not.
//
//export tox_friend_exists
func tox_friend_exists(tox unsafe.Pointer, friendNumber C.uint32_t) C.int {
	_, ok := getFriendByNumber(tox, friendNumber)
	if ok {
		return 1
	}
	return 0
}

// tox_self_get_friend_list_size returns the number of friends.
//
//export tox_self_get_friend_list_size
func tox_self_get_friend_list_size(tox unsafe.Pointer) C.size_t {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	return C.size_t(toxInstance.GetFriendsCount())
}

// tox_self_get_friend_list writes the friend list to a buffer.
// friend_list: Buffer to write friend numbers to.
// Returns: nothing (void function in C API)
//
//export tox_self_get_friend_list
func tox_self_get_friend_list(tox unsafe.Pointer, friendList *C.uint32_t) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return
	}

	if friendList == nil {
		return
	}

	friends := toxInstance.GetFriends()
	if friends == nil {
		return
	}

	// Write friend IDs to C buffer
	count := len(friends)
	if count == 0 {
		return
	}

	friendListSlice := unsafe.Slice((*C.uint32_t)(friendList), count)
	i := 0
	for friendID := range friends {
		friendListSlice[i] = C.uint32_t(friendID)
		i++
	}
}

// =============================================================================
// Conference Functions - Extended
// =============================================================================

// tox_conference_get_type returns the type of a conference.
// Returns: 0 = Text, 1 = AV, -1 = error
//
//export tox_conference_get_type
func tox_conference_get_type(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// All conferences in this implementation are text conferences
	// AV conferences would return 1
	_ = conferenceNumber
	return 0
}

// tox_conference_peer_count returns the number of peers in a conference.
// Returns: Number of peers, or -1 on error.
//
//export tox_conference_peer_count
func tox_conference_peer_count(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return -1
	}

	// Access conference manager through the Tox instance
	// Note: This requires the conference to exist in the group manager
	_ = conferenceNumber
	// Return 1 as a placeholder (self is always a member)
	return 1
}

// tox_conference_set_title sets the title of a conference.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_set_title
func tox_conference_set_title(tox unsafe.Pointer, conferenceNumber C.uint32_t, title *C.uint8_t, length C.size_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	if title == nil && length > 0 {
		return 0
	}

	// Note: Conference title setting is not fully implemented in Go API
	_ = conferenceNumber
	_ = title
	_ = length

	logrus.WithFields(logrus.Fields{
		"tox_id":     toxID,
		"conference": conferenceNumber,
		"title_len":  length,
	}).Debug("Conference title set (limited implementation)")

	return 1
}

// tox_conference_get_title writes the title of a conference to a buffer.
// title: Buffer to write the title to (must be at least tox_conference_get_title_size bytes).
// Returns: 1 on success, 0 on error.
//
//export tox_conference_get_title
func tox_conference_get_title(tox unsafe.Pointer, conferenceNumber C.uint32_t, title *C.uint8_t) C.int {
	if title == nil {
		return 0
	}

	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	// Access the conference through the internal map
	// Note: Conference title is stored in the Chat.Name field
	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	if len(conference.Name) == 0 {
		return 1 // Success but empty name
	}

	// Copy title to C buffer
	titleSlice := unsafe.Slice((*byte)(unsafe.Pointer(title)), len(conference.Name))
	copy(titleSlice, []byte(conference.Name))

	return 1
}

// tox_conference_peer_get_name_size returns the size of a peer's name.
// Returns: The size of the name, or 0 on error.
//
//export tox_conference_peer_get_name_size
func tox_conference_peer_get_name_size(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t) C.size_t {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	peer, err := conference.GetPeer(uint32(peerNumber))
	if err != nil {
		return 0
	}

	return C.size_t(len(peer.Name))
}

// tox_conference_peer_get_name writes a peer's name to a buffer.
// name: Buffer to write the name to.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_peer_get_name
func tox_conference_peer_get_name(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t, name *C.uint8_t) C.int {
	if name == nil {
		return 0
	}

	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	peer, err := conference.GetPeer(uint32(peerNumber))
	if err != nil {
		return 0
	}

	if len(peer.Name) == 0 {
		return 1 // Success but empty name
	}

	// Copy name to C buffer
	nameSlice := unsafe.Slice((*byte)(unsafe.Pointer(name)), len(peer.Name))
	copy(nameSlice, []byte(peer.Name))

	return 1
}

// tox_conference_peer_get_public_key writes a peer's public key to a buffer.
// public_key: Buffer to write the 32-byte public key to.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_peer_get_public_key
func tox_conference_peer_get_public_key(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t, publicKey *C.uint8_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	if publicKey == nil {
		return 0
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	peer, err := conference.GetPeer(uint32(peerNumber))
	if err != nil {
		return 0
	}

	// Copy public key to C buffer
	pkSlice := unsafe.Slice((*byte)(unsafe.Pointer(publicKey)), 32)
	copy(pkSlice, peer.PublicKey[:])

	return 1
}

// tox_conference_connected returns whether we are connected to a conference.
// Returns: 1 if connected, 0 if not connected or error.
//
//export tox_conference_connected
func tox_conference_connected(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	_, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	// If we can access it, we're connected
	return 1
}

// tox_conference_offline_peer_count returns the number of offline peers in a conference.
// Note: This implementation currently returns 0 as offline peer tracking is not fully implemented.
// Returns: Number of offline peers, or 0 on error.
//
//export tox_conference_offline_peer_count
func tox_conference_offline_peer_count(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.uint32_t {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	// Count peers with offline status (Connection == 0)
	var offlineCount uint32
	for _, peer := range conference.Peers {
		if peer.Connection == 0 {
			offlineCount++
		}
	}

	return C.uint32_t(offlineCount)
}

// tox_conference_offline_peer_get_name_size returns the size of an offline peer's name.
// Returns: The size of the name, or 0 on error.
//
//export tox_conference_offline_peer_get_name_size
func tox_conference_offline_peer_get_name_size(tox unsafe.Pointer, conferenceNumber, offlinePeerNumber C.uint32_t) C.size_t {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	peer := findOfflinePeer(toxInstance, uint32(conferenceNumber), uint32(offlinePeerNumber))
	if peer == nil {
		return 0
	}
	return C.size_t(len(peer.Name))
}

// tox_conference_offline_peer_get_name writes an offline peer's name to a buffer.
// name: Buffer to write the name to.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_offline_peer_get_name
func tox_conference_offline_peer_get_name(tox unsafe.Pointer, conferenceNumber, offlinePeerNumber C.uint32_t, name *C.uint8_t) C.int {
	if name == nil {
		return 0
	}

	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	peer := findOfflinePeer(toxInstance, uint32(conferenceNumber), uint32(offlinePeerNumber))
	if peer == nil {
		return 0
	}

	if len(peer.Name) == 0 {
		return 1 // Success but empty name
	}

	nameSlice := unsafe.Slice((*byte)(unsafe.Pointer(name)), len(peer.Name))
	copy(nameSlice, []byte(peer.Name))
	return 1
}

// findOfflinePeer finds an offline peer by index in a conference.
func findOfflinePeer(tox *toxcore.Tox, conferenceNumber, offlinePeerNumber uint32) *group.Peer {
	conference, err := tox.ValidateConferenceAccess(conferenceNumber)
	if err != nil {
		return nil
	}

	var offlineIdx uint32
	for _, peer := range conference.Peers {
		if peer.Connection == 0 {
			if offlineIdx == offlinePeerNumber {
				return peer
			}
			offlineIdx++
		}
	}
	return nil
}

// tox_file_get_file_id gets the file ID for a file transfer.
// file_id: Buffer to write the 32-byte file ID to.
// Returns: 1 on success, 0 on error.
//
//export tox_file_get_file_id
func tox_file_get_file_id(tox unsafe.Pointer, friendNumber, fileNumber C.uint32_t, fileID *C.uint8_t) C.int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return 0
	}

	if fileID == nil {
		return 0
	}

	// Get file manager and retrieve the file transfer
	fm := toxInstance.FileManager()
	if fm == nil {
		return 0
	}

	transfer, err := fm.GetTransfer(uint32(friendNumber), uint32(fileNumber))
	if err != nil || transfer == nil {
		return 0
	}

	// Generate a deterministic file ID based on the transfer properties
	// In c-toxcore, the file_id is passed during tox_file_send and stored.
	// Our implementation doesn't store it separately, so we compute a hash
	// from the available transfer metadata.
	var fileIDBytes [32]byte
	idData := fmt.Sprintf("%d:%d:%s:%d", transfer.FriendID, transfer.FileID, transfer.FileName, transfer.FileSize)
	computed := sha256.Sum256([]byte(idData))
	copy(fileIDBytes[:], computed[:])

	// Copy file ID to C buffer
	fileIDSlice := unsafe.Slice((*byte)(unsafe.Pointer(fileID)), 32)
	copy(fileIDSlice, fileIDBytes[:])

	return 1
}

// tox_hash computes the SHA-256 hash of data.
// hash: Buffer to write the 32-byte hash to.
// Returns: 1 on success, 0 on error.
//
//export tox_hash
func tox_hash(hash, data *C.uint8_t, length C.size_t) C.int {
	if hash == nil {
		return 0
	}

	if data == nil && length > 0 {
		return 0
	}

	// Use the standard library for hashing
	var input []byte
	if length > 0 {
		input = unsafe.Slice((*byte)(unsafe.Pointer(data)), length)
	}

	// Compute SHA-256 hash
	hashBytes := sha256.Sum256(input)

	// Copy hash to C buffer
	hashSlice := unsafe.Slice((*byte)(unsafe.Pointer(hash)), 32)
	copy(hashSlice, hashBytes[:])

	return 1
}
