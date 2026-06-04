package main

/*
#include <stdint.h>
#include <stdlib.h>

// Callback type for friend requests
typedef void (*friend_request_cb)(void *tox, const uint8_t *public_key,
                                  const uint8_t *message, size_t length, void *user_data);

// Callback type for friend messages
typedef void (*friend_message_cb)(void *tox, uint32_t friend_number,
                                  const uint8_t *message, size_t length, void *user_data);

// Callback type for friend connection status changes
typedef void (*friend_connection_status_cb)(void *tox, uint32_t friend_number,
                                           uint8_t connection_status, void *user_data);

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

// Bridge functions to invoke C callbacks from Go
static inline void invoke_friend_request_cb(friend_request_cb cb, void *tox,
                                           const uint8_t *public_key,
                                           const uint8_t *message, size_t length,
                                           void *user_data) {
    if (cb != NULL) {
        cb(tox, public_key, message, length, user_data);
    }
}

static inline void invoke_friend_message_cb(friend_message_cb cb, void *tox,
                                           uint32_t friend_number,
                                           const uint8_t *message, size_t length,
                                           void *user_data) {
    if (cb != NULL) {
        cb(tox, friend_number, message, length, user_data);
    }
}

static inline void invoke_friend_connection_status_cb(friend_connection_status_cb cb, void *tox,
                                                     uint32_t friend_number,
                                                     uint8_t connection_status,
                                                     void *user_data) {
    if (cb != NULL) {
        cb(tox, friend_number, connection_status, user_data);
    }
}

static inline void invoke_file_recv_cb(file_recv_cb cb, void *tox,
                                       uint32_t friend_number, uint32_t file_number,
                                       uint32_t kind, uint64_t file_size,
                                       const uint8_t *filename, size_t filename_length,
                                       void *user_data) {
    if (cb != NULL) {
        cb(tox, friend_number, file_number, kind, file_size, filename, filename_length, user_data);
    }
}

static inline void invoke_file_recv_chunk_cb(file_recv_chunk_cb cb, void *tox,
                                             uint32_t friend_number, uint32_t file_number,
                                             uint64_t position, const uint8_t *data,
                                             size_t length, void *user_data) {
    if (cb != NULL) {
        cb(tox, friend_number, file_number, position, data, length, user_data);
    }
}

static inline void invoke_file_chunk_request_cb(file_chunk_request_cb cb, void *tox,
                                                uint32_t friend_number, uint32_t file_number,
                                                uint64_t position, size_t length,
                                                void *user_data) {
    if (cb != NULL) {
        cb(tox, friend_number, file_number, position, length, user_data);
    }
}
*/
import "C"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
	toxcrypto "github.com/opd-ai/toxcore/crypto"
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

const (
	// C ABI semantic version for FFI consumers (Swift/Kotlin/other).
	toxABIVersionMajor  = 1
	toxABIVersionMinor  = 0
	toxABIVersionPatch  = 0
	toxABIVersionString = "1.0.0"

	// ABI feature bits advertised by tox_abi_feature_flags().
	toxABIFeatureGenerateKeypair uint64 = 1 << iota
	toxABIFeatureSecureWipe
	toxABIFeatureSafetyNumber
)

const toxABIFeatureMask = toxABIFeatureGenerateKeypair |
	toxABIFeatureSecureWipe |
	toxABIFeatureSafetyNumber

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

// liveToxHandles tracks C-malloc'd handle pointers that have not yet been freed.
// tox_kill checks this set to prevent double-free when callers (or tests) invoke
// tox_kill more than once on the same handle.
var liveToxHandles sync.Map

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

// getFriendString retrieves a derived friend string field for C API accessors.
func getFriendString(tox unsafe.Pointer, friendNumber C.uint32_t, getter func(*toxcore.Friend) string) (string, bool) {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return "", false
	}
	return getter(friend), true
}

// getFriendStringSnapshot returns a friend's string field snapshot used by both
// size and getter functions to prevent TOCTOU. Caller must not cache the result
// across multiple C API calls since the friend's profile may change.
func getFriendStringSnapshot(tox unsafe.Pointer, friendNumber C.uint32_t, getter func(*toxcore.Friend) string) (string, bool) {
	friend, ok := getFriendByNumber(tox, friendNumber)
	if !ok {
		return "", false
	}
	return getter(friend), true
}

// getConferencePeer retrieves a peer from a conference by conference and peer number.
// Returns (peer, true) if found, (nil, false) if tox instance, conference, or peer not found.
// This consolidates the common conference peer lookup pattern used across capi functions.
func getConferencePeer(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t) (*group.Peer, bool) {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return nil, false
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return nil, false
	}

	peer, err := conference.GetPeer(uint32(peerNumber))
	if err != nil {
		return nil, false
	}

	return peer, true
}

// copyStringToByteBuffer copies a Go string to a byte buffer.
// Returns 0 on success (including empty strings).
// Returns -1 on any of the following errors, consistent with the libtoxcore ABI
// convention of a single generic error sentinel for get-functions:
//   - the tox instance is not found (invalid pointer)
//   - dst is nil
//   - the string length exceeds dstCap (buffer too small)
//
// Callers that need to distinguish "invalid instance" from "buffer too small"
// should call the corresponding tox_*_size() function first and validate the
// allocated buffer before calling this function.
func copyStringToByteBuffer(tox unsafe.Pointer, dst *byte, dstCap int, getStr func(*toxcore.Tox) string) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	str := getStr(toxInstance)
	if len(str) == 0 {
		return 0
	}

	if dst == nil {
		return -1
	}
	if len(str) > dstCap {
		return -1
	}
	outputSlice := unsafe.Slice(dst, len(str))
	copy(outputSlice, []byte(str))
	return 0
}

// setStringFromByteBuffer sets a string property on a Tox instance from a C byte buffer.
// Returns 0 on success, -1 if the tox instance is not found or the setter returns an error.
// This consolidates the common pattern of receiving a C string and calling a setter.
func setStringFromByteBuffer(tox unsafe.Pointer, data *byte, dataLen int, setStr func(*toxcore.Tox, string) error) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	if data == nil && dataLen > 0 {
		return -1
	}
	var str string
	if dataLen > 0 {
		str = string(unsafe.Slice(data, dataLen))
	}
	if err := setStr(toxInstance, str); err != nil {
		return -1
	}
	return 0
}

// copyStringToCBuffer copies a Go string to a C buffer.
// Returns 1 on success (including empty strings), 0 if buffer is nil.
func copyStringToCBuffer(dst *C.uint8_t, src string) C.int {
	if dst == nil {
		return 0
	}
	if len(src) == 0 {
		return 1
	}
	dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(dst)), len(src))
	copy(dstSlice, []byte(src))
	return 1
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

	// Allocate the opaque handle in C memory so the GC cannot collect it while
	// C holds the only reference. C.malloc satisfies the cgo rule that Go
	// pointers must not be retained by C after the exported function returns.
	handle := (*int)(C.malloc(C.size_t(unsafe.Sizeof(int(0)))))
	*handle = instanceID
	// Track the live handle so tox_kill can detect and prevent double-free.
	liveToxHandles.Store(uintptr(unsafe.Pointer(handle)), struct{}{})
	return unsafe.Pointer(handle)
}

//export tox_kill
func tox_kill(tox unsafe.Pointer) {
	if tox == nil {
		return
	}
	// Atomically claim the handle; if it was not in liveToxHandles it has already
	// been freed (e.g. callers invoking tox_kill twice) — return early to prevent
	// a double-free of the C-malloc'd handle.
	if _, alive := liveToxHandles.LoadAndDelete(uintptr(tox)); !alive {
		return
	}

	toxID, ok := safeGetToxID(tox)
	if ok {
		// Clean up callback map entries for this instance
		toxCallbacksMu.Lock()
		delete(toxCallbackMap, toxID)
		toxCallbacksMu.Unlock()

		// Clean up group message and invite callbacks for this instance
		groupMessageCallbacksMu.Lock()
		delete(groupMessageCallbacks, toxID)
		groupMessageCallbacksMu.Unlock()

		groupInviteCallbacksMu.Lock()
		delete(groupInviteCallbacks, toxID)
		groupInviteCallbacksMu.Unlock()

		if toxInstance := toxRegistry.Delete(toxID); toxInstance != nil {
			toxInstance.Kill()
		}
	}
	// Free the C-malloc'd handle allocated in tox_new.
	C.free(tox)
}

//export tox_bootstrap_simple
func tox_bootstrap_simple(tox unsafe.Pointer) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	// Bootstrap with default nodes from the centralized list
	err := toxInstance.BootstrapDefaults()
	if err != nil {
		return -1
	}

	return 0 // Success
}

//export tox_iterate
func tox_iterate(tox unsafe.Pointer) {
	if toxInstance, ok := getToxFromPointer(tox); ok {
		toxInstance.Iterate()
	}
}

//export tox_iteration_interval
func tox_iteration_interval(tox unsafe.Pointer) int {
	if toxInstance, ok := getToxFromPointer(tox); ok {
		return int(toxInstance.IterationInterval().Milliseconds())
	}
	return 50 // Default 50ms
}

//export tox_self_get_address_size
func tox_self_get_address_size(tox unsafe.Pointer) int {
	if _, ok := getToxFromPointer(tox); ok {
		// Return binary address size (38 bytes = 32 public key + 4 nospam + 2 checksum)
		// c-toxcore returns 38, not the hex string length
		return 38
	}
	return 0
}

//export hex_string_to_bin
func hex_string_to_bin(hexStr *byte, hexLen int, output *byte, outputLen int) int {
	if hexStr == nil && hexLen > 0 {
		return -1
	}
	var hexBytes []byte
	if hexLen > 0 {
		hexBytes = unsafe.Slice(hexStr, hexLen)
	}
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

	if len(decoded) == 0 {
		return 0
	}
	if output == nil {
		return -1
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	addr := toxInstance.SelfGetAddress()
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		return -1
	}

	// Copy to output buffer
	if address == nil {
		return -1
	}
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	// Get address and extract public key (first 32 bytes)
	addr := toxInstance.SelfGetAddress()
	addrBytes, err := hex.DecodeString(addr)
	if err != nil || len(addrBytes) < 32 {
		return -1
	}

	// Copy public key (first 32 bytes of address)
	if publicKey == nil {
		return -1
	}
	outputSlice := unsafe.Slice(publicKey, 32)
	copy(outputSlice, addrBytes[:32])

	return 0
}

// tox_friend_add adds a friend by Tox address and sends a friend request message.
// Returns the friend number on success, or UINT32_MAX on failure.
//
//export tox_friend_add
func tox_friend_add(tox unsafe.Pointer, address, message *byte, messageLen int) uint32 {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0xFFFFFFFF
	}

	if address == nil {
		return 0xFFFFFFFF
	}
	// Convert address bytes to hex string (38 bytes = 76 hex chars)
	addrBytes := unsafe.Slice(address, 38)
	addrHex := hex.EncodeToString(addrBytes)

	// Convert message bytes to string
	var msgStr string
	if message != nil && messageLen > 0 {
		msgBytes := unsafe.Slice(message, messageLen)
		msgStr = string(msgBytes)
	}

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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0xFFFFFFFF
	}

	if publicKey == nil {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	// Convert message bytes to string
	if message == nil && messageLen > 0 {
		return 0
	}
	var msgStr string
	if messageLen > 0 {
		msgBytes := unsafe.Slice(message, messageLen)
		msgStr = string(msgBytes)
	}

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

// callbackRegistrationContext holds the resolved state needed for callback registration.
type callbackRegistrationContext struct {
	toxID       int
	toxPointer  unsafe.Pointer
	toxInstance *toxcore.Tox
	callbacks   *toxCallbacks
}

// prepareCallbackRegistration performs the common setup for all callback registration functions.
// Returns nil if validation fails (tox pointer invalid or instance not found).
func prepareCallbackRegistration(tox unsafe.Pointer) *callbackRegistrationContext {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return nil
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		return nil
	}

	return &callbackRegistrationContext{
		toxID:       toxID,
		toxPointer:  tox,
		toxInstance: toxInstance,
		callbacks:   getToxCallbacks(toxID),
	}
}

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
	ctx := prepareCallbackRegistration(tox)
	if ctx == nil {
		return
	}

	ctx.callbacks.friendRequestCb = callback
	ctx.callbacks.friendRequestUserData = userData
	toxID := ctx.toxID

	ctx.toxInstance.OnFriendRequest(func(publicKey [32]byte, message string) {
		toxCallbacksMu.RLock()
		cbData := toxCallbackMap[toxID]
		toxCallbacksMu.RUnlock()
		if cbData == nil || cbData.friendRequestCb == nil {
			return
		}
		logrus.WithFields(logrus.Fields{
			"function":   "friend_request_callback",
			"public_key": fmt.Sprintf("%x", publicKey[:8]),
			"message":    message,
		}).Debug("Friend request received (C callback registered)")

		// Call the C callback function pointer with marshal data
		pubKeyBytes := publicKey[:]
		msgBytes := []byte(message)
		var msgPtr *C.uint8_t
		if len(msgBytes) > 0 {
			msgPtr = (*C.uint8_t)(unsafe.Pointer(&msgBytes[0]))
		}
		C.invoke_friend_request_cb(
			C.friend_request_cb(cbData.friendRequestCb),
			unsafe.Pointer(ctx.toxPointer),
			(*C.uint8_t)(unsafe.Pointer(&pubKeyBytes[0])),
			msgPtr,
			C.size_t(len(msgBytes)),
			cbData.friendRequestUserData,
		)
	})
}

// tox_callback_friend_message registers a callback for friend messages.
// The callback receives: tox pointer, friend number, message type, message, message length, user data.
//
//export tox_callback_friend_message
func tox_callback_friend_message(tox, callback, userData unsafe.Pointer) {
	ctx := prepareCallbackRegistration(tox)
	if ctx == nil {
		return
	}

	ctx.callbacks.friendMessageCb = callback
	ctx.callbacks.friendMessageUserData = userData
	toxID := ctx.toxID

	ctx.toxInstance.OnFriendMessage(func(friendID uint32, message string) {
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

		// Call the C callback function pointer
		msgBytes := []byte(message)
		var msgPtr *C.uint8_t
		if len(msgBytes) > 0 {
			msgPtr = (*C.uint8_t)(unsafe.Pointer(&msgBytes[0]))
		}
		C.invoke_friend_message_cb(
			C.friend_message_cb(cbData.friendMessageCb),
			unsafe.Pointer(ctx.toxPointer),
			C.uint32_t(friendID),
			msgPtr,
			C.size_t(len(msgBytes)),
			cbData.friendMessageUserData,
		)
	})
}

// tox_callback_friend_connection_status registers a callback for friend connection status changes.
// The callback receives: tox pointer, friend number, connection status, user data.
//
//export tox_callback_friend_connection_status
func tox_callback_friend_connection_status(tox, callback, userData unsafe.Pointer) {
	ctx := prepareCallbackRegistration(tox)
	if ctx == nil {
		return
	}

	ctx.callbacks.friendConnStatusCb = callback
	ctx.callbacks.friendConnStatusData = userData
	toxID := ctx.toxID

	ctx.toxInstance.OnFriendConnectionStatus(func(friendID uint32, connectionStatus toxcore.ConnectionStatus) {
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

		// Call the C callback function pointer
		C.invoke_friend_connection_status_cb(
			C.friend_connection_status_cb(cbData.friendConnStatusCb),
			unsafe.Pointer(ctx.toxPointer),
			C.uint32_t(friendID),
			C.uint8_t(connectionStatus),
			cbData.friendConnStatusData,
		)
	})
}

// tox_self_set_name sets the name of this Tox instance.
// Returns 0 on success, -1 on error.
//
//export tox_self_set_name
func tox_self_set_name(tox unsafe.Pointer, name *byte, nameLen int) int {
	return setStringFromByteBuffer(tox, name, nameLen, func(t *toxcore.Tox, s string) error {
		return t.SelfSetName(s)
	})
}

// tox_self_get_name_size returns the length of the name.
//
//export tox_self_get_name_size
func tox_self_get_name_size(tox unsafe.Pointer) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}
	return len(toxInstance.SelfGetName())
}

// tox_self_get_name copies the name to the provided buffer.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_name
func tox_self_get_name(tox unsafe.Pointer, name *byte) int {
	return copyStringToByteBuffer(tox, name, tox_self_get_name_size(tox), func(t *toxcore.Tox) string {
		return t.SelfGetName()
	})
}

// tox_self_set_status_message sets the status message of this Tox instance.
// Returns 0 on success, -1 on error.
//
//export tox_self_set_status_message
func tox_self_set_status_message(tox unsafe.Pointer, message *byte, messageLen int) int {
	return setStringFromByteBuffer(tox, message, messageLen, func(t *toxcore.Tox, s string) error {
		return t.SelfSetStatusMessage(s)
	})
}

// tox_self_get_status_message_size returns the length of the status message.
//
//export tox_self_get_status_message_size
func tox_self_get_status_message_size(tox unsafe.Pointer) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}
	return len(toxInstance.SelfGetStatusMessage())
}

// tox_self_get_status_message copies the status message to the provided buffer.
// Returns 0 on success, -1 on error.
//
//export tox_self_get_status_message
func tox_self_get_status_message(tox unsafe.Pointer, message *byte) int {
	return copyStringToByteBuffer(tox, message, tox_self_get_status_message_size(tox), func(t *toxcore.Tox) string {
		return t.SelfGetStatusMessage()
	})
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, 1) // TOX_ERR_CONFERENCE_NEW_INIT
		return 0xFFFFFFFF
	}

	conferenceID, createErr := toxInstance.ConferenceNew()
	if createErr != nil {
		logrus.WithField("error", createErr.Error()).Error("Failed to create conference")
		setError(err, 1)
		return 0xFFFFFFFF
	}

	setError(err, 0) // TOX_ERR_CONFERENCE_NEW_OK
	return conferenceID
}

// tox_conference_invite invites a friend to a conference.
// Returns 0 on success, non-zero on error.
//
//export tox_conference_invite
func tox_conference_invite(tox unsafe.Pointer, friendID, conferenceID uint32, err *uint32) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, 1) // TOX_ERR_CONFERENCE_INVITE_CONFERENCE_NOT_FOUND
		return -1
	}

	inviteErr := toxInstance.ConferenceInvite(friendID, conferenceID)
	if inviteErr != nil {
		logrus.WithFields(logrus.Fields{
			"friend_id":     friendID,
			"conference_id": conferenceID,
			"error":         inviteErr.Error(),
		}).Error("Failed to invite friend to conference")
		setError(err, 2) // TOX_ERR_CONFERENCE_INVITE_FAIL_SEND
		return -1
	}

	setError(err, 0) // TOX_ERR_CONFERENCE_INVITE_OK
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := lookupToxInstance(tox, err)
	if !ok {
		return -1
	}

	if deleteErr := toxInstance.ConferenceDelete(conferenceID); deleteErr != nil {
		logConferenceDeleteError(conferenceID, deleteErr)
		setConferenceError(err, 1)
		return -1
	}

	setConferenceError(err, 0)
	return 0
}

// lookupToxInstance retrieves the Tox instance from a pointer.
func lookupToxInstance(tox unsafe.Pointer, err *uint32) (*toxcore.Tox, bool) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		setConferenceError(err, 1)
		return nil, false
	}

	toxInstance := toxRegistry.Get(toxID)
	if toxInstance == nil {
		setConferenceError(err, 1)
		return nil, false
	}
	return toxInstance, true
}

// logConferenceDeleteError logs a conference deletion error.
func logConferenceDeleteError(conferenceID uint32, deleteErr error) {
	logrus.WithFields(logrus.Fields{
		"conference_id": conferenceID,
		"error":         deleteErr.Error(),
	}).Error("Failed to delete conference")
}

// setConferenceError sets the error code if err is non-nil.
func setConferenceError(err *uint32, code uint32) {
	if err != nil {
		*err = code
	}
}

// setOptionalCallback updates a callback registry entry for a tox instance under the
// provided mutex, preserving thread safety for callback map reads/writes.
// Callers must pass the zero value for T (for callback function pointers, this is nil)
// because generic code cannot compare callback directly against an untyped nil.
func setOptionalCallback[T comparable](mu *sync.RWMutex, callbacks map[int]T, toxID int, callback, zeroValue T) {
	mu.Lock()
	defer mu.Unlock()

	if callback == zeroValue {
		delete(callbacks, toxID)
		return
	}
	callbacks[toxID] = callback
}

// registerToxCallback resolves a tox instance and updates the callback registry entry for it.
func registerToxCallback[T comparable](tox unsafe.Pointer, callback, zeroValue T, mu *sync.RWMutex, callbacks map[int]T) (*toxcore.Tox, int, bool) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return nil, 0, false
	}
	setOptionalCallback(mu, callbacks, toxID, callback, zeroValue)
	return toxRegistry.Get(toxID), toxID, true
}

const (
	// confTitleErrOK indicates success.
	confTitleErrOK = 0
	// confTitleErrToxNotFound indicates an invalid tox pointer.
	confTitleErrToxNotFound = 1
	// confTitleErrConferenceNotFound indicates a missing conference.
	confTitleErrConferenceNotFound = 2
)

// tox_conference_get_title gets the title of a conference.
// Returns the length of the title on success, or -1 on error.
//
//export tox_conference_get_title_size
func tox_conference_get_title_size(tox unsafe.Pointer, conferenceID uint32, err *uint32) int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		setError(err, confTitleErrToxNotFound)
		return -1
	}

	conference, conferenceErr := toxInstance.ValidateConferenceAccess(conferenceID)
	if conferenceErr != nil {
		setError(err, confTitleErrConferenceNotFound)
		return -1
	}

	setError(err, confTitleErrOK)
	return len(conference.Name)
}

// tox_callback_conference_message sets the callback for conference message events.
//
//export tox_callback_conference_message
func tox_callback_conference_message(tox unsafe.Pointer, callback C.group_message_cb) {
	_, toxID, ok := registerToxCallback(tox, callback, nil, &groupMessageCallbacksMu, groupMessageCallbacks)
	if !ok {
		return
	}

	// Note: Would need to connect this to toxcore's group message handler
	logrus.WithField("tox_id", toxID).Debug("Conference message callback registered")
}

// tox_callback_conference_invite sets the callback for conference invite events.
//
//export tox_callback_conference_invite
func tox_callback_conference_invite(tox unsafe.Pointer, callback C.group_invite_cb) {
	_, toxID, ok := registerToxCallback(tox, callback, nil, &groupInviteCallbacksMu, groupInviteCallbacks)
	if !ok {
		return
	}

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

// registerFileCallback updates a file callback registry entry and installs the matching tox hook.
func registerFileCallback[T comparable](tox unsafe.Pointer, callback, zeroValue T, mu *sync.RWMutex, callbacks map[int]T, logMessage string, register func(*toxcore.Tox, int)) {
	toxInstance, toxID, ok := registerToxCallback(tox, callback, zeroValue, mu, callbacks)
	if !ok {
		return
	}
	if toxInstance != nil {
		register(toxInstance, toxID)
	}
	logrus.WithField("tox_id", toxID).Debug(logMessage)
}

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

func invokeFileRecvCallback(cb C.file_recv_cb, tox unsafe.Pointer, friendID, fileID, kind uint32, fileSize uint64, filename string) {
	if cb == nil {
		return
	}
	filenameBytes := []byte(filename)
	var filenamePtr *C.uint8_t
	if len(filenameBytes) > 0 {
		filenamePtr = (*C.uint8_t)(unsafe.Pointer(&filenameBytes[0]))
	}
	C.invoke_file_recv_cb(
		C.file_recv_cb(cb),
		tox,
		C.uint32_t(friendID),
		C.uint32_t(fileID),
		C.uint32_t(kind),
		C.uint64_t(fileSize),
		filenamePtr,
		C.size_t(len(filenameBytes)),
		nil,
	)
	logrus.WithFields(logrus.Fields{
		"friend_id": friendID,
		"file_id":   fileID,
		"kind":      kind,
		"file_size": fileSize,
		"filename":  filename,
	}).Debug("File recv callback triggered")
}

func invokeFileRecvChunkCallback(cb C.file_recv_chunk_cb, tox unsafe.Pointer, friendID, fileID uint32, position uint64, data []byte) {
	if cb == nil {
		return
	}
	var dataPtr *C.uint8_t
	if len(data) > 0 {
		dataPtr = (*C.uint8_t)(unsafe.Pointer(&data[0]))
	}
	C.invoke_file_recv_chunk_cb(
		C.file_recv_chunk_cb(cb),
		tox,
		C.uint32_t(friendID),
		C.uint32_t(fileID),
		C.uint64_t(position),
		dataPtr,
		C.size_t(len(data)),
		nil,
	)
	logrus.WithFields(logrus.Fields{
		"friend_id": friendID,
		"file_id":   fileID,
		"position":  position,
		"data_len":  len(data),
	}).Debug("File recv chunk callback triggered")
}

func invokeFileChunkRequestCallback(cb C.file_chunk_request_cb, tox unsafe.Pointer, friendID, fileID uint32, position uint64, length int) {
	if cb == nil {
		return
	}
	if length < 0 {
		length = 0
	}
	C.invoke_file_chunk_request_cb(
		C.file_chunk_request_cb(cb),
		tox,
		C.uint32_t(friendID),
		C.uint32_t(fileID),
		C.uint64_t(position),
		C.size_t(length),
		nil,
	)
	logrus.WithFields(logrus.Fields{
		"friend_id": friendID,
		"file_id":   fileID,
		"position":  position,
		"length":    length,
	}).Debug("File chunk request callback triggered")
}

// tox_callback_file_recv sets the callback for file receive events.
//
//export tox_callback_file_recv
func tox_callback_file_recv(tox unsafe.Pointer, callback C.file_recv_cb) {
	registerFileCallback(tox, callback, nil, &fileRecvCallbacksMu, fileRecvCallbacks, "File recv callback registered", func(toxInstance *toxcore.Tox, toxID int) {
		toxInstance.OnFileRecv(func(friendID, fileID, kind uint32, fileSize uint64, filename string) {
			fileRecvCallbacksMu.RLock()
			cb := fileRecvCallbacks[toxID]
			fileRecvCallbacksMu.RUnlock()
			invokeFileRecvCallback(cb, tox, friendID, fileID, kind, fileSize, filename)
		})
	})
}

// tox_callback_file_recv_chunk sets the callback for file chunk receive events.
//
//export tox_callback_file_recv_chunk
func tox_callback_file_recv_chunk(tox unsafe.Pointer, callback C.file_recv_chunk_cb) {
	registerFileCallback(tox, callback, nil, &fileRecvChunkCallbacksMu, fileRecvChunkCallbacks, "File recv chunk callback registered", func(toxInstance *toxcore.Tox, toxID int) {
		toxInstance.OnFileRecvChunk(func(friendID, fileID uint32, position uint64, data []byte) {
			fileRecvChunkCallbacksMu.RLock()
			cb := fileRecvChunkCallbacks[toxID]
			fileRecvChunkCallbacksMu.RUnlock()
			invokeFileRecvChunkCallback(cb, tox, friendID, fileID, position, data)
		})
	})
}

// tox_callback_file_chunk_request sets the callback for file chunk request events.
//
//export tox_callback_file_chunk_request
func tox_callback_file_chunk_request(tox unsafe.Pointer, callback C.file_chunk_request_cb) {
	registerFileCallback(tox, callback, nil, &fileChunkRequestCallbacksMu, fileChunkRequestCallbacks, "File chunk request callback registered", func(toxInstance *toxcore.Tox, toxID int) {
		toxInstance.OnFileChunkRequest(func(friendID, fileID uint32, position uint64, length int) {
			fileChunkRequestCallbacksMu.RLock()
			cb := fileChunkRequestCallbacks[toxID]
			fileChunkRequestCallbacksMu.RUnlock()
			invokeFileChunkRequestCallback(cb, tox, friendID, fileID, position, length)
		})
	})
}

// =============================================================================
// Self Functions - Status and Connection
// =============================================================================

// tox_self_get_connection_status returns the connection status of the Tox instance.
// Returns: 0 = TCP, 1 = UDP, -1 = error
//
//export tox_self_get_connection_status
func tox_self_get_connection_status(tox unsafe.Pointer) C.int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}
	return C.int(toxInstance.SelfGetStatus())
}

// tox_self_set_status sets the user status of this Tox instance.
// status: 0 = None, 1 = Away, 2 = Busy
// Returns: 0 on success, -1 on error
//
//export tox_self_set_status
func tox_self_set_status(tox unsafe.Pointer, status C.int) C.int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	if err := setSelfStatusFromC(toxInstance, status); err != nil {
		return -1
	}
	return 0
}

func setSelfStatusFromC(toxInstance *toxcore.Tox, status C.int) error {
	if status < 0 {
		return fmt.Errorf("invalid status: %d", status)
	}
	return toxInstance.SelfSetStatus(toxcore.UserStatus(status))
}

// tox_self_get_nospam returns the 4-byte nospam value from the Tox ID.
//
//export tox_self_get_nospam
func tox_self_get_nospam(tox unsafe.Pointer) C.uint32_t {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	name, ok := getFriendStringSnapshot(tox, friendNumber, func(friend *toxcore.Friend) string { return friend.Name })
	if !ok {
		return 0
	}
	return C.size_t(len(name))
}

// tox_friend_get_name writes a friend's name to a buffer.
// name: Buffer to write the name to (must be at least tox_friend_get_name_size bytes).
// WARNING: This function follows the libtoxcore size-then-copy pattern. If the name changes
// between the tox_friend_get_name_size() call and this function, the buffer may overflow.
// Callers must synchronize the size+copy call pair as one logical operation when the
// peer's profile may change concurrently.
// Returns: 1 on success, 0 on error.
//
//export tox_friend_get_name
func tox_friend_get_name(tox unsafe.Pointer, friendNumber C.uint32_t, name *C.uint8_t) C.int {
	friendName, ok := getFriendStringSnapshot(tox, friendNumber, func(friend *toxcore.Friend) string { return friend.Name })
	if !ok {
		return 0
	}
	return copyStringToCBuffer(name, friendName)
}

// tox_friend_get_status_message_size returns the length of a friend's status message.
// Returns: The length of the status message, or 0 on error.
//
//export tox_friend_get_status_message_size
func tox_friend_get_status_message_size(tox unsafe.Pointer, friendNumber C.uint32_t) C.size_t {
	statusMessage, ok := getFriendStringSnapshot(tox, friendNumber, func(friend *toxcore.Friend) string { return friend.StatusMessage })
	if !ok {
		return 0
	}
	return C.size_t(len(statusMessage))
}

// tox_friend_get_status_message writes a friend's status message to a buffer.
// status_message: Buffer to write the status message to.
// WARNING: This function follows the libtoxcore size-then-copy pattern. If the status message changes
// between the tox_friend_get_status_message_size() call and this function, the buffer may overflow.
// Callers must synchronize the size+copy call pair as one logical operation when the
// peer's profile may change concurrently.
// Returns: 1 on success, 0 on error.
//
//export tox_friend_get_status_message
func tox_friend_get_status_message(tox unsafe.Pointer, friendNumber C.uint32_t, statusMessage *C.uint8_t) C.int {
	friendStatus, ok := getFriendStringSnapshot(tox, friendNumber, func(friend *toxcore.Friend) string { return friend.StatusMessage })
	if !ok {
		return 0
	}
	return copyStringToCBuffer(statusMessage, friendStatus)
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}
	return C.size_t(toxInstance.GetFriendsCount())
}

// tox_self_get_friend_list writes the friend list to a buffer.
// friend_list: Buffer to write friend numbers to.
// Returns: nothing (void function in C API)
//
// NOTE: This API does not receive the caller's buffer length. The caller must
// allocate from tox_self_get_friend_list_size() and synchronize that size/list
// sequence externally; concurrent friend additions can otherwise overflow.
//
//export tox_self_get_friend_list
func tox_self_get_friend_list(tox unsafe.Pointer, friendList *C.uint32_t) {
	if friendList == nil {
		return
	}

	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return
	}

	// Use GetFriends() snapshot consistently to avoid count/list mismatches within
	// this call; caller-side synchronization still governs buffer safety.
	friends := toxInstance.GetFriends()
	if friends == nil || len(friends) == 0 {
		return
	}

	// Write friend IDs to C buffer
	friendListSlice := unsafe.Slice((*C.uint32_t)(friendList), len(friends))
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
// Returns: 0 = Text, 1 = AV, -1 = error.
//
//export tox_conference_get_type
func tox_conference_get_type(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return -1
	}

	switch conference.Type {
	case group.ChatTypeText:
		return 0
	case group.ChatTypeAV:
		return 1
	default:
		return -1
	}
}

// tox_conference_peer_count returns the number of peers in a conference.
// Returns: Number of peers, or -1 on error.
//
//export tox_conference_peer_count
func tox_conference_peer_count(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return -1
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return -1
	}

	return C.int(len(conference.Peers))
}

// tox_conference_set_title sets the title of a conference.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_set_title
func tox_conference_set_title(tox unsafe.Pointer, conferenceNumber C.uint32_t, title *C.uint8_t, length C.size_t) C.int {
	titleStr, err := conferenceTitleString(title, length)
	if err != nil {
		return 0
	}
	if setErr := setConferenceTitle(tox, conferenceNumber, titleStr); setErr != nil {
		return 0
	}
	return 1
}

func conferenceTitleString(title *C.uint8_t, length C.size_t) (string, error) {
	if title == nil && length > 0 {
		return "", errors.New("nil title with positive length")
	}
	if length == 0 {
		return "", nil
	}
	titleBytes := unsafe.Slice((*byte)(unsafe.Pointer(title)), int(length))
	return string(titleBytes), nil
}

func setConferenceTitle(tox unsafe.Pointer, conferenceNumber C.uint32_t, titleStr string) error {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return fmt.Errorf("tox instance not found")
	}
	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return err
	}
	return conference.SetName(titleStr)
}

// tox_conference_get_title writes the title of a conference to a buffer.
// title: Buffer to write the title to (must be at least tox_conference_get_title_size bytes).
// Returns: 1 on success, 0 on error.
//
//export tox_conference_get_title
func tox_conference_get_title(tox unsafe.Pointer, conferenceNumber C.uint32_t, title *C.uint8_t) C.int {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	conference, err := toxInstance.ValidateConferenceAccess(uint32(conferenceNumber))
	if err != nil {
		return 0
	}

	return copyStringToCBuffer(title, conference.Name)
}

// tox_conference_peer_get_name_size returns the size of a peer's name.
// Returns: The size of the name, or 0 on error.
//
//export tox_conference_peer_get_name_size
func tox_conference_peer_get_name_size(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t) C.size_t {
	peer, ok := getConferencePeer(tox, conferenceNumber, peerNumber)
	if !ok {
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
	peer, ok := getConferencePeer(tox, conferenceNumber, peerNumber)
	if !ok {
		return 0
	}
	return copyStringToCBuffer(name, peer.Name)
}

// tox_conference_peer_get_public_key writes a peer's public key to a buffer.
// public_key: Buffer to write the 32-byte public key to.
// Returns: 1 on success, 0 on error.
//
//export tox_conference_peer_get_public_key
func tox_conference_peer_get_public_key(tox unsafe.Pointer, conferenceNumber, peerNumber C.uint32_t, publicKey *C.uint8_t) C.int {
	if publicKey == nil {
		return 0
	}

	peer, ok := getConferencePeer(tox, conferenceNumber, peerNumber)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
// Returns: Number of offline peers, or 0 on error.
//
//export tox_conference_offline_peer_count
func tox_conference_offline_peer_count(tox unsafe.Pointer, conferenceNumber C.uint32_t) C.uint32_t {
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
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
	toxInstance, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	peer := findOfflinePeer(toxInstance, uint32(conferenceNumber), uint32(offlinePeerNumber))
	if peer == nil {
		return 0
	}

	return copyStringToCBuffer(name, peer.Name)
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

// tox_abi_version_major returns the C ABI major version.
//
//export tox_abi_version_major
func tox_abi_version_major() C.uint32_t {
	return C.uint32_t(toxABIVersionMajor)
}

// tox_abi_version_minor returns the C ABI minor version.
//
//export tox_abi_version_minor
func tox_abi_version_minor() C.uint32_t {
	return C.uint32_t(toxABIVersionMinor)
}

// tox_abi_version_patch returns the C ABI patch version.
//
//export tox_abi_version_patch
func tox_abi_version_patch() C.uint32_t {
	return C.uint32_t(toxABIVersionPatch)
}

// tox_abi_version_string writes the ABI semantic version string (e.g. "1.0.0").
// out may be nil to query required size. Returns string length without terminator,
// or 0 on error.
//
//export tox_abi_version_string
func tox_abi_version_string(out *byte, outLen int) int {
	needed := len(toxABIVersionString) + 1 // include NUL terminator

	if out == nil {
		return len(toxABIVersionString)
	}
	if outLen < needed {
		return 0
	}

	outSlice := unsafe.Slice(out, outLen)
	copy(outSlice, toxABIVersionString)
	outSlice[len(toxABIVersionString)] = 0

	return len(toxABIVersionString)
}

// tox_abi_feature_flags returns a bitmask of security-critical ABI features.
//
//export tox_abi_feature_flags
func tox_abi_feature_flags() C.uint64_t {
	return C.uint64_t(toxABIFeatureMask)
}

// tox_crypto_generate_keypair creates a new Curve25519 key pair.
// publicKey and secretKey must each point to writable 32-byte buffers.
// Returns 1 on success, 0 on error.
//
//export tox_crypto_generate_keypair
func tox_crypto_generate_keypair(publicKey, secretKey *byte) int {
	if publicKey == nil || secretKey == nil {
		return 0
	}

	kp, err := toxcrypto.GenerateKeyPair()
	if err != nil {
		return 0
	}
	defer toxcrypto.WipeKeyPair(kp)

	pubSlice := unsafe.Slice(publicKey, toxcrypto.KeySize)
	secSlice := unsafe.Slice(secretKey, toxcrypto.KeySize)
	copy(pubSlice, kp.Public[:])
	copy(secSlice, kp.Private[:])

	return 1
}

// tox_crypto_secure_wipe clears a mutable byte buffer in-place.
// Returns 1 on success, 0 on error.
//
//export tox_crypto_secure_wipe
func tox_crypto_secure_wipe(data *byte, length int) int {
	if data == nil {
		if length == 0 {
			return 1
		}
		return 0
	}

	b := unsafe.Slice(data, length)
	if err := toxcrypto.SecureWipe(b); err != nil {
		return 0
	}

	return 1
}

// tox_self_get_safety_number derives the 60-digit safety number for this Tox
// instance and a peer public key.
//
// peerPublicKey must point to a 32-byte key.
// out may be nil to query required size.
// Returns the string length (without null terminator) on success, 0 on error.
//
//export tox_self_get_safety_number
func tox_self_get_safety_number(tox unsafe.Pointer, peerPublicKey, out *byte, outLen int) int {
	if peerPublicKey == nil {
		return 0
	}

	toxi, ok := getToxFromPointer(tox)
	if !ok {
		return 0
	}

	var peerPK [toxcrypto.KeySize]byte
	copy(peerPK[:], unsafe.Slice(peerPublicKey, toxcrypto.KeySize))

	sn := toxi.SafetyNumber(peerPK)
	needed := len(sn) + 1 // include null terminator for C callers

	if out == nil {
		return len(sn)
	}
	if outLen < needed {
		return 0
	}

	outSlice := unsafe.Slice(out, outLen)
	copy(outSlice, sn)
	outSlice[len(sn)] = 0

	return len(sn)
}
