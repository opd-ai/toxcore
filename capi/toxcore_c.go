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
	"encoding/hex"
	"fmt"
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
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
//export tox_conference_send_message
func tox_conference_send_message(tox unsafe.Pointer, conferenceID uint32, msgType int, message *byte, length uint32, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_CONFERENCE_SEND_MESSAGE_CONFERENCE_NOT_FOUND
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

	// Convert C string to Go string
	if message == nil || length == 0 {
		if err != nil {
			*err = 3 // TOX_ERR_CONFERENCE_SEND_MESSAGE_NO_CONNECTION
		}
		return -1
	}

	messageSlice := unsafe.Slice(message, length)
	messageStr := string(messageSlice)

	// Convert message type
	toxMsgType := toxcore.MessageTypeNormal
	if msgType == 1 {
		toxMsgType = toxcore.MessageTypeAction
	}

	sendErr := toxInstance.ConferenceSendMessage(conferenceID, messageStr, toxMsgType)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"conference_id": conferenceID,
			"error":         sendErr.Error(),
		}).Error("Failed to send conference message")
		if err != nil {
			*err = 3 // TOX_ERR_CONFERENCE_SEND_MESSAGE_NO_CONNECTION
		}
		return -1
	}

	if err != nil {
		*err = 0 // TOX_ERR_CONFERENCE_SEND_MESSAGE_OK
	}
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
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_FILE_SEND_NULL
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

	// Convert filename from C string to Go string
	var filenameStr string
	if filename != nil && filenameLen > 0 {
		filenameSlice := unsafe.Slice(filename, filenameLen)
		filenameStr = string(filenameSlice)
	}

	// Convert file ID from C array to Go array
	var fileIDArray [32]byte
	if fileID != nil {
		fileIDSlice := unsafe.Slice(fileID, 32)
		copy(fileIDArray[:], fileIDSlice)
	}

	fileNum, sendErr := toxInstance.FileSend(friendID, kind, fileSize, fileIDArray, filenameStr)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id": friendID,
			"filename":  filenameStr,
			"error":     sendErr.Error(),
		}).Error("Failed to send file")
		if err != nil {
			*err = 6 // TOX_ERR_FILE_SEND_FRIEND_NOT_CONNECTED
		}
		return 0xFFFFFFFF
	}

	if err != nil {
		*err = 0 // TOX_ERR_FILE_SEND_OK
	}
	return fileNum
}

// tox_file_control controls an ongoing file transfer.
// Returns 0 on success, non-zero on error.
//
//export tox_file_control
func tox_file_control(tox unsafe.Pointer, friendID, fileID uint32, control int, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_FILE_CONTROL_FRIEND_NOT_FOUND
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

	// Convert control to toxcore.FileControl
	var fileControl toxcore.FileControl
	switch control {
	case 0:
		fileControl = toxcore.FileControlResume
	case 1:
		fileControl = toxcore.FileControlPause
	case 2:
		fileControl = toxcore.FileControlCancel
	default:
		if err != nil {
			*err = 5 // TOX_ERR_FILE_CONTROL_SENDQ
		}
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
		if err != nil {
			*err = 4 // TOX_ERR_FILE_CONTROL_NOT_FOUND
		}
		return -1
	}

	if err != nil {
		*err = 0 // TOX_ERR_FILE_CONTROL_OK
	}
	return 0
}

// tox_file_send_chunk sends a chunk of a file being transferred.
// Returns 0 on success, non-zero on error.
//
//export tox_file_send_chunk
func tox_file_send_chunk(tox unsafe.Pointer, friendID, fileID uint32, position uint64, data *byte, length uint32, err *uint32) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		if err != nil {
			*err = 1 // TOX_ERR_FILE_SEND_CHUNK_NULL
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

	// Convert data to Go slice
	var dataSlice []byte
	if data != nil && length > 0 {
		dataSlice = unsafe.Slice(data, length)
	}

	sendErr := toxInstance.FileSendChunk(friendID, fileID, position, dataSlice)
	if sendErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id": friendID,
			"file_id":   fileID,
			"position":  position,
			"length":    length,
			"error":     sendErr.Error(),
		}).Error("Failed to send file chunk")
		if err != nil {
			*err = 5 // TOX_ERR_FILE_SEND_CHUNK_SENDQ
		}
		return -1
	}

	if err != nil {
		*err = 0 // TOX_ERR_FILE_SEND_CHUNK_OK
	}
	return 0
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
