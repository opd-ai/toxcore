package c

/*
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>

// Forward declarations for callback function types
typedef struct Tox Tox;
typedef enum TOX_ERR_NEW {
    TOX_ERR_NEW_OK = 0,
    TOX_ERR_NEW_NULL,
    TOX_ERR_NEW_MALLOC,
    TOX_ERR_NEW_PORT_ALLOC,
    TOX_ERR_NEW_PROXY_BAD_TYPE,
    TOX_ERR_NEW_PROXY_BAD_HOST,
    TOX_ERR_NEW_PROXY_BAD_PORT,
    TOX_ERR_NEW_PROXY_NOT_FOUND,
    TOX_ERR_NEW_LOAD_ENCRYPTED,
    TOX_ERR_NEW_LOAD_BAD_FORMAT
} TOX_ERR_NEW;

typedef enum TOX_ERR_FRIEND_ADD {
    TOX_ERR_FRIEND_ADD_OK = 0,
    TOX_ERR_FRIEND_ADD_NULL,
    TOX_ERR_FRIEND_ADD_TOO_LONG,
    TOX_ERR_FRIEND_ADD_NO_MESSAGE,
    TOX_ERR_FRIEND_ADD_OWN_KEY,
    TOX_ERR_FRIEND_ADD_ALREADY_SENT,
    TOX_ERR_FRIEND_ADD_BAD_CHECKSUM,
    TOX_ERR_FRIEND_ADD_SET_NEW_NOSPAM,
    TOX_ERR_FRIEND_ADD_MALLOC
} TOX_ERR_FRIEND_ADD;

typedef enum TOX_ERR_FRIEND_SEND_MESSAGE {
    TOX_ERR_FRIEND_SEND_MESSAGE_OK = 0,
    TOX_ERR_FRIEND_SEND_MESSAGE_NULL,
    TOX_ERR_FRIEND_SEND_MESSAGE_FRIEND_NOT_FOUND,
    TOX_ERR_FRIEND_SEND_MESSAGE_FRIEND_NOT_CONNECTED,
    TOX_ERR_FRIEND_SEND_MESSAGE_SENDQ,
    TOX_ERR_FRIEND_SEND_MESSAGE_TOO_LONG,
    TOX_ERR_FRIEND_SEND_MESSAGE_EMPTY
} TOX_ERR_FRIEND_SEND_MESSAGE;

typedef enum TOX_ERR_FILE_SEND {
    TOX_ERR_FILE_SEND_OK = 0,
    TOX_ERR_FILE_SEND_NULL,
    TOX_ERR_FILE_SEND_FRIEND_NOT_FOUND,
    TOX_ERR_FILE_SEND_FRIEND_NOT_CONNECTED,
    TOX_ERR_FILE_SEND_NAME_TOO_LONG,
    TOX_ERR_FILE_SEND_TOO_MANY
} TOX_ERR_FILE_SEND;

typedef enum TOX_MESSAGE_TYPE {
    TOX_MESSAGE_TYPE_NORMAL,
    TOX_MESSAGE_TYPE_ACTION
} TOX_MESSAGE_TYPE;

// Callback function types
typedef void (*tox_self_connection_status_cb)(int connection_status, void* user_data);
typedef void (*tox_friend_request_cb)(uint8_t* public_key, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_friend_message_cb)(uint32_t friend_number, int message_type, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_friend_name_cb)(uint32_t friend_number, uint8_t* name, size_t length, void* user_data);
typedef void (*tox_friend_status_cb)(uint32_t friend_number, int status, void* user_data);
typedef void (*tox_friend_status_message_cb)(uint32_t friend_number, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_friend_connection_status_cb)(uint32_t friend_number, int connection_status, void* user_data);
typedef void (*tox_friend_typing_cb)(uint32_t friend_number, bool is_typing, void* user_data);
typedef void (*tox_friend_read_receipt_cb)(uint32_t friend_number, uint32_t message_id, void* user_data);
typedef void (*tox_file_recv_control_cb)(uint32_t friend_number, uint32_t file_number, int control, void* user_data);
typedef void (*tox_file_chunk_request_cb)(uint32_t friend_number, uint32_t file_number, uint64_t position, size_t length, void* user_data);
typedef void (*tox_file_recv_cb)(uint32_t friend_number, uint32_t file_number, uint32_t kind, uint64_t file_size, uint8_t* filename, size_t filename_length, void* user_data);
typedef void (*tox_file_recv_chunk_cb)(uint32_t friend_number, uint32_t file_number, uint64_t position, uint8_t* data, size_t length, void* user_data);
typedef void (*tox_conference_invite_cb)(uint32_t friend_number, int type, uint8_t* cookie, size_t length, void* user_data);
typedef void (*tox_conference_message_cb)(uint32_t conference_number, uint32_t peer_number, int type, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_conference_peer_name_cb)(uint32_t conference_number, uint32_t peer_number, uint8_t* name, size_t length, void* user_data);
typedef void (*tox_conference_peer_list_changed_cb)(uint32_t conference_number, void* user_data);
*/
import "C"

import (
	"log"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/opd-ai/toxcore"
)

// Global registry to manage Tox instances and prevent garbage collection
var (
	toxInstances     = make(map[unsafe.Pointer]*toxcore.Tox)
	toxInstancesLock sync.RWMutex
	nextInstanceID   uintptr = 1
)

// Callback registries to prevent callbacks from being garbage collected
type callbacks struct {
	selfConnectionStatus      C.tox_self_connection_status_cb
	friendRequest             C.tox_friend_request_cb
	friendMessage             C.tox_friend_message_cb
	friendName                C.tox_friend_name_cb
	friendStatus              C.tox_friend_status_cb
	friendStatusMessage       C.tox_friend_status_message_cb
	friendConnectionStatus    C.tox_friend_connection_status_cb
	friendTyping              C.tox_friend_typing_cb
	friendReadReceipt         C.tox_friend_read_receipt_cb
	fileRecvControl           C.tox_file_recv_control_cb
	fileChunkRequest          C.tox_file_chunk_request_cb
	fileRecv                  C.tox_file_recv_cb
	fileRecvChunk             C.tox_file_recv_chunk_cb
	conferenceInvite          C.tox_conference_invite_cb
	conferenceMessage         C.tox_conference_message_cb
	conferencePeerName        C.tox_conference_peer_name_cb
	conferencePeerListChanged C.tox_conference_peer_list_changed_cb
	userData                  unsafe.Pointer
}

var callbackRegistry = make(map[unsafe.Pointer]*callbacks)

// Helper functions for instance management
func registerToxInstance(tox *toxcore.Tox) unsafe.Pointer {
	toxInstancesLock.Lock()
	defer toxInstancesLock.Unlock()

	instancePtr := unsafe.Pointer(uintptr(nextInstanceID))
	nextInstanceID++

	toxInstances[instancePtr] = tox
	callbackRegistry[instancePtr] = &callbacks{}

	return instancePtr
}

func getToxInstance(instancePtr unsafe.Pointer) *toxcore.Tox {
	toxInstancesLock.RLock()
	defer toxInstancesLock.RUnlock()

	return toxInstances[instancePtr]
}

func getCallbacks(instancePtr unsafe.Pointer) *callbacks {
	toxInstancesLock.RLock()
	defer toxInstancesLock.RUnlock()

	return callbackRegistry[instancePtr]
}

func unregisterToxInstance(instancePtr unsafe.Pointer) {
	toxInstancesLock.Lock()
	defer toxInstancesLock.Unlock()

	delete(toxInstances, instancePtr)
	delete(callbackRegistry, instancePtr)
}

//export ToxNew
func ToxNew(options unsafe.Pointer, errorOut *C.TOX_ERR_NEW) unsafe.Pointer {
	opts := toxcore.NewOptions()
	if options != nil {
		// Convert C options to Go options
		// In a real implementation, this would parse the C options struct
	}

	tox, err := toxcore.New(opts)
	if err != nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_NEW_MALLOC
		}
		return nil
	}

	if errorOut != nil {
		*errorOut = C.TOX_ERR_NEW_OK
	}
	return registerToxInstance(tox)
}

//export ToxKill
func ToxKill(toxPtr unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	tox.Kill()
	unregisterToxInstance(toxPtr)
}

//export ToxGetSavedata
func ToxGetSavedata(toxPtr unsafe.Pointer, saveData unsafe.Pointer) C.size_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || saveData == nil {
		return 0
	}

	data := tox.GetSavedata()
	if len(data) == 0 {
		return 0
	}

	// Copy data to provided buffer
	dataSlice := (*[1 << 30]byte)(saveData)
	for i, b := range data {
		dataSlice[i] = b
	}

	return C.size_t(len(data))
}

//export ToxGetSavedataSize
func ToxGetSavedataSize(toxPtr unsafe.Pointer) C.size_t {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return 0
	}

	return C.size_t(len(tox.GetSavedata()))
}

//export ToxBootstrap
func ToxBootstrap(toxPtr unsafe.Pointer, address *C.char, port C.uint16_t, publicKey *C.uint8_t) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil || address == nil || publicKey == nil {
		return C.bool(false)
	}

	var pubKey [32]byte
	for i := 0; i < 32; i++ {
		pubKey[i] = byte(publicKey[i])
	}

	err := tox.Bootstrap(C.GoString(address), uint16(port), pubKey)
	return C.bool(err == nil)
}

//export ToxIterationInterval
func ToxIterationInterval(toxPtr unsafe.Pointer) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return 50 // Default interval in milliseconds
	}

	return C.uint32_t(tox.IterationInterval() / time.Millisecond)
}

//export ToxIterate
func ToxIterate(toxPtr unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	tox.Iterate()
}

// Friend functions

//export ToxFriendAdd
func ToxFriendAdd(toxPtr unsafe.Pointer, address *C.uint8_t, message *C.uint8_t, length C.size_t, errorOut *C.TOX_ERR_FRIEND_ADD) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || address == nil || message == nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FRIEND_ADD_NULL
		}
		return C.uint32_t(^uint32(0)) // MAX_UINT32 as error value
	}

	var addr [32]byte
	for i := 0; i < 32; i++ {
		addr[i] = byte(address[i])
	}

	msg := C.GoBytes(unsafe.Pointer(message), C.int(length))

	friendID, err := tox.AddFriend(addr, string(msg))
	if err != nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FRIEND_ADD_MALLOC // Default error
		}
		return C.uint32_t(^uint32(0))
	}

	if errorOut != nil {
		*errorOut = C.TOX_ERR_FRIEND_ADD_OK
	}
	return C.uint32_t(friendID)
}

//export ToxSelfGetAddress
func ToxSelfGetAddress(toxPtr unsafe.Pointer, address *C.uint8_t) {
	tox := getToxInstance(toxPtr)
	if tox == nil || address == nil {
		return
	}

	addr := tox.SelfGetAddress()
	for i, b := range addr {
		address[i] = C.uint8_t(b)
	}
}

//export ToxFriendByPublicKey
func ToxFriendByPublicKey(toxPtr unsafe.Pointer, publicKey *C.uint8_t, errorOut *C.int) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || publicKey == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.uint32_t(^uint32(0))
	}

	var pubKey [32]byte
	for i := 0; i < 32; i++ {
		pubKey[i] = byte(publicKey[i])
	}

	friendID, err := tox.FriendByPublicKey(pubKey)
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.uint32_t(^uint32(0))
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.uint32_t(friendID)
}

//export ToxFriendDelete
func ToxFriendDelete(toxPtr unsafe.Pointer, friendNumber C.uint32_t, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	err := tox.DeleteFriend(uint32(friendNumber))
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxFriendGetPublicKey
func ToxFriendGetPublicKey(toxPtr unsafe.Pointer, friendNumber C.uint32_t, publicKey *C.uint8_t, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil || publicKey == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	pubKey, err := tox.FriendGetPublicKey(uint32(friendNumber))
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	for i, b := range pubKey {
		publicKey[i] = C.uint8_t(b)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxFriendSendMessage
func ToxFriendSendMessage(toxPtr unsafe.Pointer, friendNumber C.uint32_t, messageType C.TOX_MESSAGE_TYPE,
	message *C.uint8_t, length C.size_t, errorOut *C.TOX_ERR_FRIEND_SEND_MESSAGE,
) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || message == nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FRIEND_SEND_MESSAGE_NULL
		}
		return 0
	}

	msg := C.GoBytes(unsafe.Pointer(message), C.int(length))

	var msgType toxcore.MessageType
	if messageType == C.TOX_MESSAGE_TYPE_ACTION {
		msgType = toxcore.MessageTypeAction
	} else {
		msgType = toxcore.MessageTypeNormal
	}

	msgID, err := tox.FriendSendMessage(uint32(friendNumber), string(msg), msgType)
	if err != nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FRIEND_SEND_MESSAGE_FRIEND_NOT_FOUND
		}
		return 0
	}

	if errorOut != nil {
		*errorOut = C.TOX_ERR_FRIEND_SEND_MESSAGE_OK
	}
	return C.uint32_t(msgID)
}

//export ToxSelfSetName
func ToxSelfSetName(toxPtr unsafe.Pointer, name *C.uint8_t, length C.size_t, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil || name == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	nameBytes := C.GoBytes(unsafe.Pointer(name), C.int(length))

	err := tox.SelfSetName(string(nameBytes))
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxSelfGetName
func ToxSelfGetName(toxPtr unsafe.Pointer, name *C.uint8_t) C.size_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || name == nil {
		return 0
	}

	selfName := tox.SelfGetName()
	nameBytes := []byte(selfName)

	for i, b := range nameBytes {
		name[i] = C.uint8_t(b)
	}

	return C.size_t(len(nameBytes))
}

//export ToxSelfSetStatusMessage
func ToxSelfSetStatusMessage(toxPtr unsafe.Pointer, message *C.uint8_t, length C.size_t, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil || message == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	msgBytes := C.GoBytes(unsafe.Pointer(message), C.int(length))

	err := tox.SelfSetStatusMessage(string(msgBytes))
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxSelfGetStatusMessage
func ToxSelfGetStatusMessage(toxPtr unsafe.Pointer, message *C.uint8_t) C.size_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || message == nil {
		return 0
	}

	statusMsg := tox.SelfGetStatusMessage()
	msgBytes := []byte(statusMsg)

	for i, b := range msgBytes {
		message[i] = C.uint8_t(b)
	}

	return C.size_t(len(msgBytes))
}

//export ToxFileControl
func ToxFileControl(toxPtr unsafe.Pointer, friendNumber C.uint32_t, fileNumber C.uint32_t, control C.int, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	// Convert C control code to Go enum
	var controlCode toxcore.FileControl
	switch control {
	case 0:
		controlCode = toxcore.FileControlResume
	case 1:
		controlCode = toxcore.FileControlPause
	case 2:
		controlCode = toxcore.FileControlCancel
	default:
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	err := tox.FileControl(uint32(friendNumber), uint32(fileNumber), controlCode)
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxFileSend
func ToxFileSend(toxPtr unsafe.Pointer, friendNumber C.uint32_t, kind C.uint32_t, fileSize C.uint64_t,
	fileId *C.uint8_t, fileName *C.uint8_t, fileNameLength C.size_t, errorOut *C.TOX_ERR_FILE_SEND,
) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil || fileName == nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FILE_SEND_NULL
		}
		return C.uint32_t(^uint32(0))
	}

	var fileIDBytes [32]byte
	if fileId != nil {
		for i := 0; i < 32; i++ {
			fileIDBytes[i] = byte(fileId[i])
		}
	}

	nameBytes := C.GoBytes(unsafe.Pointer(fileName), C.int(fileNameLength))

	fileNum, err := tox.FileSend(uint32(friendNumber), uint32(kind), uint64(fileSize), fileIDBytes, string(nameBytes))
	if err != nil {
		if errorOut != nil {
			*errorOut = C.TOX_ERR_FILE_SEND_FRIEND_NOT_CONNECTED
		}
		return C.uint32_t(^uint32(0))
	}

	if errorOut != nil {
		*errorOut = C.TOX_ERR_FILE_SEND_OK
	}
	return C.uint32_t(fileNum)
}

//export ToxFileSendChunk
func ToxFileSendChunk(toxPtr unsafe.Pointer, friendNumber C.uint32_t, fileNumber C.uint32_t, position C.uint64_t,
	data *C.uint8_t, length C.size_t, errorOut *C.int,
) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	var chunk []byte
	if length > 0 && data != nil {
		chunk = C.GoBytes(unsafe.Pointer(data), C.int(length))
	} else {
		chunk = []byte{}
	}

	err := tox.FileSendChunk(uint32(friendNumber), uint32(fileNumber), uint64(position), chunk)
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

// Callback registration functions

//export ToxCallbackFriendRequest
func ToxCallbackFriendRequest(toxPtr unsafe.Pointer, callback C.tox_friend_request_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.friendRequest = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		if cbs.friendRequest != nil {
			msgBytes := []byte(message)
			cMsg := (*C.uint8_t)(C.malloc(C.size_t(len(msgBytes))))
			defer C.free(unsafe.Pointer(cMsg))

			// Copy message to C memory
			cMsgSlice := (*[1 << 30]C.uint8_t)(unsafe.Pointer(cMsg))[:len(msgBytes):len(msgBytes)]
			for i, b := range msgBytes {
				cMsgSlice[i] = C.uint8_t(b)
			}

			// Convert public key to C format
			var cPubKey [32]C.uint8_t
			for i, b := range publicKey {
				cPubKey[i] = C.uint8_t(b)
			}

			cbs.friendRequest((*C.uint8_t)(&cPubKey[0]), cMsg, C.size_t(len(msgBytes)), cbs.userData)
		}
	})
}

//export ToxCallbackFriendMessage
func ToxCallbackFriendMessage(toxPtr unsafe.Pointer, callback C.tox_friend_message_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.friendMessage = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFriendMessage(func(friendNumber uint32, message string, messageType toxcore.MessageType) {
		if cbs.friendMessage != nil {
			msgBytes := []byte(message)
			cMsg := (*C.uint8_t)(C.malloc(C.size_t(len(msgBytes))))
			defer C.free(unsafe.Pointer(cMsg))

			// Copy message to C memory
			cMsgSlice := (*[1 << 30]C.uint8_t)(unsafe.Pointer(cMsg))[:len(msgBytes):len(msgBytes)]
			for i, b := range msgBytes {
				cMsgSlice[i] = C.uint8_t(b)
			}

			var cMsgType C.int
			if messageType == toxcore.MessageTypeAction {
				cMsgType = C.TOX_MESSAGE_TYPE_ACTION
			} else {
				cMsgType = C.TOX_MESSAGE_TYPE_NORMAL
			}

			cbs.friendMessage(C.uint32_t(friendNumber), cMsgType, cMsg, C.size_t(len(msgBytes)), cbs.userData)
		}
	})
}

//export ToxCallbackFriendName
func ToxCallbackFriendName(toxPtr unsafe.Pointer, callback C.tox_friend_name_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.friendName = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFriendName(func(friendNumber uint32, name string) {
		if cbs.friendName != nil {
			nameBytes := []byte(name)
			cName := (*C.uint8_t)(C.malloc(C.size_t(len(nameBytes))))
			defer C.free(unsafe.Pointer(cName))

			// Copy name to C memory
			cNameSlice := (*[1 << 30]C.uint8_t)(unsafe.Pointer(cName))[:len(nameBytes):len(nameBytes)]
			for i, b := range nameBytes {
				cNameSlice[i] = C.uint8_t(b)
			}

			cbs.friendName(C.uint32_t(friendNumber), cName, C.size_t(len(nameBytes)), cbs.userData)
		}
	})
}

//export ToxCallbackFileRecv
func ToxCallbackFileRecv(toxPtr unsafe.Pointer, callback C.tox_file_recv_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.fileRecv = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFileRecv(func(friendNumber uint32, fileNumber uint32, kind uint32, fileSize uint64, filename string) {
		if cbs.fileRecv != nil {
			filenameBytes := []byte(filename)
			cFilename := (*C.uint8_t)(C.malloc(C.size_t(len(filenameBytes))))
			defer C.free(unsafe.Pointer(cFilename))

			// Copy filename to C memory
			cFilenameSlice := (*[1 << 30]C.uint8_t)(unsafe.Pointer(cFilename))[:len(filenameBytes):len(filenameBytes)]
			for i, b := range filenameBytes {
				cFilenameSlice[i] = C.uint8_t(b)
			}

			cbs.fileRecv(
				C.uint32_t(friendNumber),
				C.uint32_t(fileNumber),
				C.uint32_t(kind),
				C.uint64_t(fileSize),
				cFilename,
				C.size_t(len(filenameBytes)),
				cbs.userData,
			)
		}
	})
}

//export ToxCallbackFileRecvChunk
func ToxCallbackFileRecvChunk(toxPtr unsafe.Pointer, callback C.tox_file_recv_chunk_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.fileRecvChunk = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFileRecvChunk(func(friendNumber uint32, fileNumber uint32, position uint64, data []byte) {
		if cbs.fileRecvChunk != nil {
			var cData *C.uint8_t

			if len(data) > 0 {
				cData = (*C.uint8_t)(C.malloc(C.size_t(len(data))))
				defer C.free(unsafe.Pointer(cData))

				// Copy data to C memory
				cDataSlice := (*[1 << 30]C.uint8_t)(unsafe.Pointer(cData))[:len(data):len(data)]
				for i, b := range data {
					cDataSlice[i] = C.uint8_t(b)
				}
			}

			cbs.fileRecvChunk(
				C.uint32_t(friendNumber),
				C.uint32_t(fileNumber),
				C.uint64_t(position),
				cData,
				C.size_t(len(data)),
				cbs.userData,
			)
		}
	})
}

//export ToxCallbackFileChunkRequest
func ToxCallbackFileChunkRequest(toxPtr unsafe.Pointer, callback C.tox_file_chunk_request_cb, userData unsafe.Pointer) {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		return
	}

	cbs := getCallbacks(toxPtr)
	if cbs == nil {
		return
	}

	// Store the callback in our registry
	cbs.fileChunkRequest = callback
	cbs.userData = userData

	// Set up the Go callback
	tox.OnFileChunkRequest(func(friendNumber uint32, fileNumber uint32, position uint64, length int) {
		if cbs.fileChunkRequest != nil {
			cbs.fileChunkRequest(
				C.uint32_t(friendNumber),
				C.uint32_t(fileNumber),
				C.uint64_t(position),
				C.size_t(length),
				cbs.userData,
			)
		}
	})
}

// Group/Conference chat bindings
// These would be expanded in a real implementation

//export ToxConferenceNew
func ToxConferenceNew(toxPtr unsafe.Pointer, errorOut *C.int) C.uint32_t {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.uint32_t(^uint32(0))
	}

	conferenceID, err := tox.ConferenceNew()
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.uint32_t(^uint32(0))
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.uint32_t(conferenceID)
}

//export ToxConferenceInvite
func ToxConferenceInvite(toxPtr unsafe.Pointer, friendNumber C.uint32_t, conferenceNumber C.uint32_t, errorOut *C.int) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	err := tox.ConferenceInvite(uint32(friendNumber), uint32(conferenceNumber))
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

//export ToxConferenceSendMessage
func ToxConferenceSendMessage(toxPtr unsafe.Pointer, conferenceNumber C.uint32_t, messageType C.int,
	message *C.uint8_t, length C.size_t, errorOut *C.int,
) C.bool {
	tox := getToxInstance(toxPtr)
	if tox == nil || message == nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	msgBytes := C.GoBytes(unsafe.Pointer(message), C.int(length))

	var msgType toxcore.MessageType
	if messageType == 1 {
		msgType = toxcore.MessageTypeAction
	} else {
		msgType = toxcore.MessageTypeNormal
	}

	err := tox.ConferenceSendMessage(uint32(conferenceNumber), string(msgBytes), msgType)
	if err != nil {
		if errorOut != nil {
			*errorOut = 1 // Error
		}
		return C.bool(false)
	}

	if errorOut != nil {
		*errorOut = 0 // Success
	}
	return C.bool(true)
}

// Utility function to create a C header file for the bindings
func generateHeader() {
	header := `// Generated by toxcore-go bindings
#ifndef _TOXCORE_GO_H
#define _TOXCORE_GO_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct Tox Tox;

// Error enums
typedef enum TOX_ERR_NEW {
	TOX_ERR_NEW_OK = 0,
	TOX_ERR_NEW_NULL,
	TOX_ERR_NEW_MALLOC,
	TOX_ERR_NEW_PORT_ALLOC,
	TOX_ERR_NEW_PROXY_BAD_TYPE,
	TOX_ERR_NEW_PROXY_BAD_HOST,
	TOX_ERR_NEW_PROXY_BAD_PORT,
	TOX_ERR_NEW_PROXY_NOT_FOUND,
	TOX_ERR_NEW_LOAD_ENCRYPTED,
	TOX_ERR_NEW_LOAD_BAD_FORMAT
} TOX_ERR_NEW;

typedef enum TOX_ERR_FRIEND_ADD {
	TOX_ERR_FRIEND_ADD_OK = 0,
	TOX_ERR_FRIEND_ADD_NULL,
	TOX_ERR_FRIEND_ADD_TOO_LONG,
	TOX_ERR_FRIEND_ADD_NO_MESSAGE,
	TOX_ERR_FRIEND_ADD_OWN_KEY,
	TOX_ERR_FRIEND_ADD_ALREADY_SENT,
	TOX_ERR_FRIEND_ADD_BAD_CHECKSUM,
	TOX_ERR_FRIEND_ADD_SET_NEW_NOSPAM,
	TOX_ERR_FRIEND_ADD_MALLOC
} TOX_ERR_FRIEND_ADD;

typedef enum TOX_MESSAGE_TYPE {
	TOX_MESSAGE_TYPE_NORMAL,
	TOX_MESSAGE_TYPE_ACTION
} TOX_MESSAGE_TYPE;

// Callback function types
typedef void (*tox_self_connection_status_cb)(int connection_status, void* user_data);
typedef void (*tox_friend_request_cb)(uint8_t* public_key, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_friend_message_cb)(uint32_t friend_number, int message_type, uint8_t* message, size_t length, void* user_data);
typedef void (*tox_friend_name_cb)(uint32_t friend_number, uint8_t* name, size_t length, void* user_data);
typedef void (*tox_file_recv_cb)(uint32_t friend_number, uint32_t file_number, uint32_t kind, uint64_t file_size, uint8_t* filename, size_t filename_length, void* user_data);
typedef void (*tox_file_recv_chunk_cb)(uint32_t friend_number, uint32_t file_number, uint64_t position, uint8_t* data, size_t length, void* user_data);
typedef void (*tox_file_chunk_request_cb)(uint32_t friend_number, uint32_t file_number, uint64_t position, size_t length, void* user_data);

// Core Functions
Tox* ToxNew(void* options, TOX_ERR_NEW* error);
void ToxKill(Tox* tox);
size_t ToxGetSavedata(Tox* tox, uint8_t* savedata);
size_t ToxGetSavedataSize(Tox* tox);
bool ToxBootstrap(Tox* tox, const char* address, uint16_t port, const uint8_t* public_key);
uint32_t ToxIterationInterval(Tox* tox);
void ToxIterate(Tox* tox);

// Friend Functions
uint32_t ToxFriendAdd(Tox* tox, const uint8_t* address, const uint8_t* message, size_t length, TOX_ERR_FRIEND_ADD* error);
void ToxSelfGetAddress(Tox* tox, uint8_t* address);
uint32_t ToxFriendByPublicKey(Tox* tox, const uint8_t* public_key, int* error);
bool ToxFriendDelete(Tox* tox, uint32_t friend_number, int* error);
bool ToxFriendGetPublicKey(Tox* tox, uint32_t friend_number, uint8_t* public_key, int* error);
uint32_t ToxFriendSendMessage(Tox* tox, uint32_t friend_number, TOX_MESSAGE_TYPE type, const uint8_t* message, size_t length, int* error);

// Self Functions
bool ToxSelfSetName(Tox* tox, const uint8_t* name, size_t length, int* error);
size_t ToxSelfGetName(Tox* tox, uint8_t* name);
bool ToxSelfSetStatusMessage(Tox* tox, const uint8_t* message, size_t length, int* error);
size_t ToxSelfGetStatusMessage(Tox* tox, uint8_t* message);

// File Transfer Functions
bool ToxFileControl(Tox* tox, uint32_t friend_number, uint32_t file_number, int control, int* error);
uint32_t ToxFileSend(Tox* tox, uint32_t friend_number, uint32_t kind, uint64_t file_size, const uint8_t* file_id, const uint8_t* filename, size_t filename_length, int* error);
bool ToxFileSendChunk(Tox* tox, uint32_t friend_number, uint32_t file_number, uint64_t position, const uint8_t* data, size_t length, int* error);

// Conference Functions
uint32_t ToxConferenceNew(Tox* tox, int* error);
bool ToxConferenceInvite(Tox* tox, uint32_t friend_number, uint32_t conference_number, int* error);
bool ToxConferenceSendMessage(Tox* tox, uint32_t conference_number, int type, const uint8_t* message, size_t length, int* error);

// Callback Registration Functions
void ToxCallbackFriendRequest(Tox* tox, tox_friend_request_cb callback, void* user_data);
void ToxCallbackFriendMessage(Tox* tox, tox_friend_message_cb callback, void* user_data);
void ToxCallbackFriendName(Tox* tox, tox_friend_name_cb callback, void* user_data);
void ToxCallbackFileRecv(Tox* tox, tox_file_recv_cb callback, void* user_data);
void ToxCallbackFileRecvChunk(Tox* tox, tox_file_recv_chunk_cb callback, void* user_data);
void ToxCallbackFileChunkRequest(Tox* tox, tox_file_chunk_request_cb callback, void* user_data);

#ifdef __cplusplus
}
#endif

#endif // _TOXCORE_GO_H
`

	// Write the header to a file
	err := os.WriteFile("toxcore_go.h", []byte(header), 0o644)
	if err != nil {
		log.Printf("Error writing header file: %v", err)
		return
	}
	log.Println("Successfully generated toxcore_go.h")
}
