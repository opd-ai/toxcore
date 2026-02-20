// Package main provides C API bindings for ToxAV functionality.
//
// This package creates a C-compatible API that matches the libtoxcore
// ToxAV interface exactly, enabling seamless integration with existing
// C applications and language bindings.
//
// The C API follows the established patterns from the main toxcore
// C bindings and provides identical function signatures and behavior
// to the original libtoxcore ToxAV implementation.
//
// Build instructions:
//
//	go build -buildmode=c-shared -o libtoxav.so capi/toxav_c.go
//
// This will be implemented in Phase 1 as part of the core infrastructure.
package main

/*
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

// Forward declarations matching libtoxcore ToxAV API
typedef struct ToxAV ToxAV;
typedef struct Tox Tox;

// Call state enum matching libtoxcore
typedef enum TOX_AV_CALL_STATE {
    TOX_AV_CALL_STATE_NONE = 0,
    TOX_AV_CALL_STATE_ERROR = 1,
    TOX_AV_CALL_STATE_FINISHED = 2,
    TOX_AV_CALL_STATE_SENDING_AUDIO = 4,
    TOX_AV_CALL_STATE_SENDING_VIDEO = 8,
    TOX_AV_CALL_STATE_ACCEPTING_AUDIO = 16,
    TOX_AV_CALL_STATE_ACCEPTING_VIDEO = 32,
} TOX_AV_CALL_STATE;

// Call control enum matching libtoxcore
typedef enum TOX_AV_CALL_CONTROL {
    TOX_AV_CALL_CONTROL_RESUME = 0,
    TOX_AV_CALL_CONTROL_PAUSE = 1,
    TOX_AV_CALL_CONTROL_CANCEL = 2,
    TOX_AV_CALL_CONTROL_MUTE_AUDIO = 3,
    TOX_AV_CALL_CONTROL_UNMUTE_AUDIO = 4,
    TOX_AV_CALL_CONTROL_HIDE_VIDEO = 5,
    TOX_AV_CALL_CONTROL_SHOW_VIDEO = 6,
} TOX_AV_CALL_CONTROL;

// Error enums matching libtoxcore
typedef enum TOX_AV_ERR_NEW {
    TOX_AV_ERR_NEW_OK = 0,
    TOX_AV_ERR_NEW_NULL = 1,
    TOX_AV_ERR_NEW_MALLOC = 2,
    TOX_AV_ERR_NEW_MULTIPLE = 3,
} TOX_AV_ERR_NEW;

typedef enum TOX_AV_ERR_CALL {
    TOX_AV_ERR_CALL_OK = 0,
    TOX_AV_ERR_CALL_MALLOC = 1,
    TOX_AV_ERR_CALL_SYNC = 2,
    TOX_AV_ERR_CALL_FRIEND_NOT_FOUND = 3,
    TOX_AV_ERR_CALL_FRIEND_NOT_CONNECTED = 4,
    TOX_AV_ERR_CALL_FRIEND_ALREADY_IN_CALL = 5,
    TOX_AV_ERR_CALL_INVALID_BIT_RATE = 6,
} TOX_AV_ERR_CALL;

typedef enum TOX_AV_ERR_ANSWER {
    TOX_AV_ERR_ANSWER_OK = 0,
    TOX_AV_ERR_ANSWER_SYNC = 1,
    TOX_AV_ERR_ANSWER_CODEC_INITIALIZATION = 2,
    TOX_AV_ERR_ANSWER_FRIEND_NOT_FOUND = 3,
    TOX_AV_ERR_ANSWER_FRIEND_NOT_CALLING = 4,
    TOX_AV_ERR_ANSWER_INVALID_BIT_RATE = 5,
} TOX_AV_ERR_ANSWER;

typedef enum TOX_AV_ERR_CALL_CONTROL {
    TOX_AV_ERR_CALL_CONTROL_OK = 0,
    TOX_AV_ERR_CALL_CONTROL_SYNC = 1,
    TOX_AV_ERR_CALL_CONTROL_FRIEND_NOT_FOUND = 2,
    TOX_AV_ERR_CALL_CONTROL_FRIEND_NOT_IN_CALL = 3,
    TOX_AV_ERR_CALL_CONTROL_INVALID_TRANSITION = 4,
} TOX_AV_ERR_CALL_CONTROL;

typedef enum TOX_AV_ERR_BIT_RATE_SET {
    TOX_AV_ERR_BIT_RATE_SET_OK = 0,
    TOX_AV_ERR_BIT_RATE_SET_SYNC = 1,
    TOX_AV_ERR_BIT_RATE_SET_INVALID_AUDIO_BIT_RATE = 2,
    TOX_AV_ERR_BIT_RATE_SET_INVALID_VIDEO_BIT_RATE = 3,
    TOX_AV_ERR_BIT_RATE_SET_FRIEND_NOT_FOUND = 4,
    TOX_AV_ERR_BIT_RATE_SET_FRIEND_NOT_IN_CALL = 5,
} TOX_AV_ERR_BIT_RATE_SET;

typedef enum TOX_AV_ERR_SEND_FRAME {
    TOX_AV_ERR_SEND_FRAME_OK = 0,
    TOX_AV_ERR_SEND_FRAME_NULL = 1,
    TOX_AV_ERR_SEND_FRAME_FRIEND_NOT_FOUND = 2,
    TOX_AV_ERR_SEND_FRAME_FRIEND_NOT_IN_CALL = 3,
    TOX_AV_ERR_SEND_FRAME_SYNC = 4,
    TOX_AV_ERR_SEND_FRAME_INVALID = 5,
    TOX_AV_ERR_SEND_FRAME_PAYLOAD_TYPE_DISABLED = 6,
    TOX_AV_ERR_SEND_FRAME_RTP_FAILED = 7,
} TOX_AV_ERR_SEND_FRAME;

// Callback function types matching libtoxcore exactly
typedef void (*toxav_call_cb)(ToxAV *av, uint32_t friend_number, bool audio_enabled, bool video_enabled, void *user_data);
typedef void (*toxav_call_state_cb)(ToxAV *av, uint32_t friend_number, uint32_t state, void *user_data);
typedef void (*toxav_audio_bit_rate_cb)(ToxAV *av, uint32_t friend_number, uint32_t audio_bit_rate, void *user_data);
typedef void (*toxav_video_bit_rate_cb)(ToxAV *av, uint32_t friend_number, uint32_t video_bit_rate, void *user_data);
typedef void (*toxav_audio_receive_frame_cb)(ToxAV *av, uint32_t friend_number, const int16_t *pcm, size_t sample_count, uint8_t channels, uint32_t sampling_rate, void *user_data);
typedef void (*toxav_video_receive_frame_cb)(ToxAV *av, uint32_t friend_number, uint16_t width, uint16_t height, const uint8_t *y, const uint8_t *u, const uint8_t *v, int32_t ystride, int32_t ustride, int32_t vstride, void *user_data);

// Bridge functions to invoke C callbacks from Go
// These are necessary because Go cannot directly call C function pointers

static inline void invoke_call_cb(toxav_call_cb cb, ToxAV *av, uint32_t friend_number, bool audio_enabled, bool video_enabled, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, audio_enabled, video_enabled, user_data);
    }
}

static inline void invoke_call_state_cb(toxav_call_state_cb cb, ToxAV *av, uint32_t friend_number, uint32_t state, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, state, user_data);
    }
}

static inline void invoke_audio_bit_rate_cb(toxav_audio_bit_rate_cb cb, ToxAV *av, uint32_t friend_number, uint32_t audio_bit_rate, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, audio_bit_rate, user_data);
    }
}

static inline void invoke_video_bit_rate_cb(toxav_video_bit_rate_cb cb, ToxAV *av, uint32_t friend_number, uint32_t video_bit_rate, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, video_bit_rate, user_data);
    }
}

static inline void invoke_audio_receive_frame_cb(toxav_audio_receive_frame_cb cb, ToxAV *av, uint32_t friend_number, const int16_t *pcm, size_t sample_count, uint8_t channels, uint32_t sampling_rate, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, pcm, sample_count, channels, sampling_rate, user_data);
    }
}

static inline void invoke_video_receive_frame_cb(toxav_video_receive_frame_cb cb, ToxAV *av, uint32_t friend_number, uint16_t width, uint16_t height, const uint8_t *y, const uint8_t *u, const uint8_t *v, int32_t ystride, int32_t ustride, int32_t vstride, void *user_data) {
    if (cb != NULL) {
        cb(av, friend_number, width, height, y, u, v, ystride, ustride, vstride, user_data);
    }
}
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
	avpkg "github.com/opd-ai/toxcore/av"
	"github.com/sirupsen/logrus"
)

// contains is a helper function for case-insensitive substring matching in error messages.
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// getToxIDFromPointer extracts the Tox instance ID from an opaque C pointer.
// The pointer comes from toxcore_c.go's tox_new function.
// Returns (id, valid) where valid indicates if the pointer points to a real Tox instance.
func getToxIDFromPointer(ptr unsafe.Pointer) (int, bool) {
	if ptr == nil {
		return 0, false
	}

	// Use defer/recover to catch segfaults from invalid pointers
	var toxID int
	var validDeref bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Invalid pointer caused a panic during dereference
				validDeref = false
				logrus.WithFields(logrus.Fields{
					"function": "getToxIDFromPointer",
					"error":    r,
				}).Warn("Invalid pointer dereference caught")
			}
		}()

		// The pointer is actually a pointer to an int (the instance ID)
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

// getToxInstance retrieves a Tox instance by ID using the authorized accessor.
// This function bridges toxav_c.go to toxcore_c.go's instance management
// while respecting encapsulation and thread-safety.
func getToxInstance(toxID int) *toxcore.Tox {
	return GetToxInstanceByID(toxID)
}

// Global instance management for C API compatibility
// This follows the same pattern as the main toxcore C bindings
var (
	toxavInstances         = make(map[uintptr]*toxcore.ToxAV)
	toxavToTox             = make(map[uintptr]unsafe.Pointer) // Maps ToxAV ID to Tox pointer
	toxavHandles           = make(map[uintptr]unsafe.Pointer) // Maps ToxAV ID to opaque handle pointer
	nextToxAVID    uintptr = 1
	toxavMutex     sync.RWMutex
)

// toxavCallbacks stores C callback function pointers and user data for each ToxAV instance.
//
// Each ToxAV instance maintains its own set of callback registrations through this struct.
// The callback function pointers (e.g., callCb, callStateCb) are C function pointers that
// match the libtoxcore ToxAV callback signatures. The corresponding userData fields store
// opaque pointers that are passed back to the C callbacks when invoked.
//
// This struct enables the Go implementation to invoke user-registered C callbacks in the
// same manner as the original libtoxcore implementation, maintaining API compatibility.
//
// Fields:
//   - callCb/callUserData: Incoming call notification callback
//   - callStateCb/callStateUserData: Call state change callback
//   - audioBitRateCb/audioBitRateUserData: Audio bitrate suggestion callback
//   - videoBitRateCb/videoBitRateUserData: Video bitrate suggestion callback
//   - audioReceiveFrameCb/audioReceiveUserData: Audio frame reception callback
//   - videoReceiveFrameCb/videoReceiveUserData: Video frame reception callback
type toxavCallbacks struct {
	callCb               C.toxav_call_cb
	callUserData         unsafe.Pointer
	callStateCb          C.toxav_call_state_cb
	callStateUserData    unsafe.Pointer
	audioBitRateCb       C.toxav_audio_bit_rate_cb
	audioBitRateUserData unsafe.Pointer
	videoBitRateCb       C.toxav_video_bit_rate_cb
	videoBitRateUserData unsafe.Pointer
	audioReceiveFrameCb  C.toxav_audio_receive_frame_cb
	audioReceiveUserData unsafe.Pointer
	videoReceiveFrameCb  C.toxav_video_receive_frame_cb
	videoReceiveUserData unsafe.Pointer
}

// toxavCallbackStorage maps ToxAV instance IDs to their callback storage
var toxavCallbackStorage = make(map[uintptr]*toxavCallbacks)

// getToxAVID safely extracts the toxavID from an opaque pointer handle
func getToxAVID(av unsafe.Pointer) (uintptr, bool) {
	if av == nil {
		return 0, false
	}
	handle := (*uintptr)(av)
	return *handle, true
}

// toxav_new creates a new ToxAV instance from a Tox instance.
//
// This function matches the libtoxcore toxav_new API exactly.
//
//export toxav_new
func toxav_new(tox unsafe.Pointer, error_ptr *C.TOX_AV_ERR_NEW) unsafe.Pointer {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_NEW_OK
	}

	if tox == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_NEW_NULL
		}
		return nil
	}

	// Extract the Tox instance ID from the opaque pointer
	// The tox pointer is an opaque handle to a Tox instance from toxcore_c.go
	toxID, ok := getToxIDFromPointer(tox)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_NEW_NULL
		}
		return nil
	}

	// Get the Tox instance from toxcore_c.go's instance map
	toxInstance := getToxInstance(toxID)
	if toxInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_NEW_NULL
		}
		return nil
	}

	// Create a new ToxAV instance from the Tox instance
	toxavInstance, err := toxcore.NewToxAV(toxInstance)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "toxav_new",
			"error":    err.Error(),
		}).Error("Failed to create ToxAV instance")
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_NEW_MALLOC
		}
		return nil
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID := nextToxAVID
	nextToxAVID++

	// Store the ToxAV instance and map it to the Tox pointer
	toxavInstances[toxavID] = toxavInstance
	toxavToTox[toxavID] = tox

	// Initialize callback storage for this instance
	toxavCallbackStorage[toxavID] = &toxavCallbacks{}

	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_NEW_OK
	}

	// Create a real memory allocation to use as an opaque pointer
	handle := new(uintptr)
	*handle = toxavID

	// Store the handle pointer for use in callbacks
	toxavHandles[toxavID] = unsafe.Pointer(handle)

	logrus.WithFields(logrus.Fields{
		"function": "toxav_new",
		"toxav_id": toxavID,
		"tox_ptr":  tox,
	}).Info("ToxAV instance created successfully")

	return unsafe.Pointer(handle)
}

// toxav_kill gracefully shuts down a ToxAV instance.
//
// This function matches the libtoxcore toxav_kill API exactly.
//
//export toxav_kill
func toxav_kill(av unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists {
		if toxavInstance != nil {
			toxavInstance.Kill()
		}
		delete(toxavInstances, toxavID)
		delete(toxavToTox, toxavID)
		delete(toxavHandles, toxavID)
		delete(toxavCallbackStorage, toxavID)
	}
}

// toxav_get_tox_from_av returns the Tox instance associated with ToxAV.
//
// This function matches the libtoxcore toxav_get_tox_from_av API exactly.
//
//export toxav_get_tox_from_av
func toxav_get_tox_from_av(av unsafe.Pointer) unsafe.Pointer {
	if av == nil {
		return nil
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return nil
	}

	// Return the original Tox pointer that was used to create this ToxAV instance
	if toxPtr, exists := toxavToTox[toxavID]; exists {
		return toxPtr
	}

	return nil
}

// toxav_iteration_interval returns the iteration interval for ToxAV.
//
// This function matches the libtoxcore toxav_iteration_interval API exactly.
//
//export toxav_iteration_interval
func toxav_iteration_interval(av unsafe.Pointer) C.uint32_t {
	if av == nil {
		return 20 // Default 20ms interval
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return 20 // Default 20ms interval
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		return C.uint32_t(toxavInstance.IterationInterval().Milliseconds())
	}
	return 20 // Default 20ms interval
}

// toxav_iterate performs one iteration of the ToxAV event loop.
//
// This function matches the libtoxcore toxav_iterate API exactly.
//
//export toxav_iterate
func toxav_iterate(av unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		toxavInstance.Iterate()
	}
}

// mapCallError maps a Go error to the appropriate C call error code.
func mapCallError(err error, error_ptr *C.TOX_AV_ERR_CALL) {
	if error_ptr == nil {
		return
	}
	errStr := err.Error()
	if contains(errStr, "not found") {
		*error_ptr = C.TOX_AV_ERR_CALL_FRIEND_NOT_FOUND
	} else if contains(errStr, "not connected") {
		*error_ptr = C.TOX_AV_ERR_CALL_FRIEND_NOT_CONNECTED
	} else if contains(errStr, "already in call") {
		*error_ptr = C.TOX_AV_ERR_CALL_FRIEND_ALREADY_IN_CALL
	} else if contains(errStr, "bit rate") || contains(errStr, "invalid") {
		*error_ptr = C.TOX_AV_ERR_CALL_INVALID_BIT_RATE
	} else {
		*error_ptr = C.TOX_AV_ERR_CALL_SYNC
	}
}

// mapAnswerError maps a Go error to the appropriate C answer error code.
func mapAnswerError(err error, error_ptr *C.TOX_AV_ERR_ANSWER) {
	if error_ptr == nil {
		return
	}
	errStr := err.Error()
	if contains(errStr, "not found") {
		*error_ptr = C.TOX_AV_ERR_ANSWER_FRIEND_NOT_FOUND
	} else if contains(errStr, "not calling") || contains(errStr, "no pending") {
		*error_ptr = C.TOX_AV_ERR_ANSWER_FRIEND_NOT_CALLING
	} else if contains(errStr, "bit rate") || contains(errStr, "invalid") {
		*error_ptr = C.TOX_AV_ERR_ANSWER_INVALID_BIT_RATE
	} else if contains(errStr, "codec") {
		*error_ptr = C.TOX_AV_ERR_ANSWER_CODEC_INITIALIZATION
	} else {
		*error_ptr = C.TOX_AV_ERR_ANSWER_SYNC
	}
}

// toxav_call initiates an audio/video call.
//
// This function matches the libtoxcore toxav_call API exactly.
//
//export toxav_call
func toxav_call(av unsafe.Pointer, friend_number, audio_bit_rate, video_bit_rate C.uint32_t, error_ptr *C.TOX_AV_ERR_CALL) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_CALL_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_SYNC
		}
		return C.bool(false)
	}

	err := toxavInstance.Call(uint32(friend_number), uint32(audio_bit_rate), uint32(video_bit_rate))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":       "toxav_call",
			"friend_number":  friend_number,
			"audio_bit_rate": audio_bit_rate,
			"video_bit_rate": video_bit_rate,
			"error":          err.Error(),
		}).Warn("Failed to initiate call")
		mapCallError(err, error_ptr)
		return C.bool(false)
	}
	return C.bool(true)
}

// toxav_answer accepts an incoming audio/video call.
//
// This function matches the libtoxcore toxav_answer API exactly.
//
//export toxav_answer
func toxav_answer(av unsafe.Pointer, friend_number, audio_bit_rate, video_bit_rate C.uint32_t, error_ptr *C.TOX_AV_ERR_ANSWER) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_ANSWER_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_ANSWER_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_ANSWER_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_ANSWER_SYNC
		}
		return C.bool(false)
	}

	err := toxavInstance.Answer(uint32(friend_number), uint32(audio_bit_rate), uint32(video_bit_rate))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":       "toxav_answer",
			"friend_number":  friend_number,
			"audio_bit_rate": audio_bit_rate,
			"video_bit_rate": video_bit_rate,
			"error":          err.Error(),
		}).Warn("Failed to answer call")
		mapAnswerError(err, error_ptr)
		return C.bool(false)
	}
	return C.bool(true)
}

// mapCallControlError maps a Go error to the appropriate C call control error code.
func mapCallControlError(err error, error_ptr *C.TOX_AV_ERR_CALL_CONTROL) {
	if error_ptr == nil {
		return
	}
	errStr := err.Error()
	if contains(errStr, "not found") {
		*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_FRIEND_NOT_FOUND
	} else if contains(errStr, "not in call") {
		*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_FRIEND_NOT_IN_CALL
	} else if contains(errStr, "invalid") || contains(errStr, "transition") {
		*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_INVALID_TRANSITION
	} else {
		*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_SYNC
	}
}

// mapBitRateSetError maps a Go error to the appropriate C bit rate error code for audio.
func mapBitRateSetError(err error, error_ptr *C.TOX_AV_ERR_BIT_RATE_SET, isAudio bool) {
	if error_ptr == nil {
		return
	}
	errStr := err.Error()
	if contains(errStr, "not found") {
		*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_FRIEND_NOT_FOUND
	} else if contains(errStr, "not in call") {
		*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_FRIEND_NOT_IN_CALL
	} else if contains(errStr, "invalid") || contains(errStr, "bit rate") {
		if isAudio {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_INVALID_AUDIO_BIT_RATE
		} else {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_INVALID_VIDEO_BIT_RATE
		}
	} else {
		*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
	}
}

// toxav_call_control sends a call control command.
//
// This function matches the libtoxcore toxav_call_control API exactly.
//
//export toxav_call_control
func toxav_call_control(av unsafe.Pointer, friend_number C.uint32_t, control C.TOX_AV_CALL_CONTROL, error_ptr *C.TOX_AV_ERR_CALL_CONTROL) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_CALL_CONTROL_SYNC
		}
		return C.bool(false)
	}

	goControl := avpkg.CallControl(control)
	err := toxavInstance.CallControl(uint32(friend_number), goControl)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "toxav_call_control",
			"friend_number": friend_number,
			"control":       control,
			"error":         err.Error(),
		}).Warn("Failed to send call control")
		mapCallControlError(err, error_ptr)
		return C.bool(false)
	}
	return C.bool(true)
}

// toxav_audio_set_bit_rate sets the audio bit rate for a call.
//
// This function matches the libtoxcore toxav_audio_set_bit_rate API exactly.
//
//export toxav_audio_set_bit_rate
func toxav_audio_set_bit_rate(av unsafe.Pointer, friend_number, bit_rate C.uint32_t, error_ptr *C.TOX_AV_ERR_BIT_RATE_SET) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	err := toxavInstance.AudioSetBitRate(uint32(friend_number), uint32(bit_rate))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "toxav_audio_set_bit_rate",
			"friend_number": friend_number,
			"bit_rate":      bit_rate,
			"error":         err.Error(),
		}).Warn("Failed to set audio bit rate")
		mapBitRateSetError(err, error_ptr, true)
		return C.bool(false)
	}
	return C.bool(true)
}

// toxav_video_set_bit_rate sets the video bit rate for a call.
//
// This function matches the libtoxcore toxav_video_set_bit_rate API exactly.
//
//export toxav_video_set_bit_rate
func toxav_video_set_bit_rate(av unsafe.Pointer, friend_number, bit_rate C.uint32_t, error_ptr *C.TOX_AV_ERR_BIT_RATE_SET) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_BIT_RATE_SET_SYNC
		}
		return C.bool(false)
	}

	err := toxavInstance.VideoSetBitRate(uint32(friend_number), uint32(bit_rate))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "toxav_video_set_bit_rate",
			"friend_number": friend_number,
			"bit_rate":      bit_rate,
			"error":         err.Error(),
		}).Warn("Failed to set video bit rate")
		mapBitRateSetError(err, error_ptr, false)
		return C.bool(false)
	}
	return C.bool(true)
}

// validateAudioFrameParams validates audio frame input parameters.
func validateAudioFrameParams(pcm *C.int16_t, totalSamples int, error_ptr *C.TOX_AV_ERR_SEND_FRAME) bool {
	if pcm == nil || totalSamples <= 0 {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_NULL
		}
		return false
	}

	const maxSamples = 1 << 20
	if totalSamples > maxSamples {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_INVALID
		}
		return false
	}
	return true
}

// convertPCMToSlice converts C PCM data to a Go slice.
func convertPCMToSlice(pcm *C.int16_t, totalSamples int) []int16 {
	return (*[1 << 20]int16)(unsafe.Pointer(pcm))[:totalSamples:totalSamples]
}

// mapSendFrameError maps a Go error to the appropriate C error code.
func mapSendFrameError(err error, error_ptr *C.TOX_AV_ERR_SEND_FRAME) {
	if error_ptr == nil {
		return
	}
	errStr := err.Error()
	if contains(errStr, "not found") {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_FRIEND_NOT_FOUND
	} else if contains(errStr, "not in call") {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_FRIEND_NOT_IN_CALL
	} else if contains(errStr, "disabled") {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_PAYLOAD_TYPE_DISABLED
	} else if contains(errStr, "rtp") || contains(errStr, "send") {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_RTP_FAILED
	} else {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
	}
}

// toxav_audio_send_frame sends an audio frame.
//
// This function matches the libtoxcore toxav_audio_send_frame API exactly.
//
//export toxav_audio_send_frame
func toxav_audio_send_frame(av unsafe.Pointer, friend_number C.uint32_t, pcm *C.int16_t, sample_count C.size_t, channels C.uint8_t, sampling_rate C.uint32_t, error_ptr *C.TOX_AV_ERR_SEND_FRAME) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	sampleCountInt := int(sample_count)
	channelsInt := int(channels)
	totalSamples := sampleCountInt * channelsInt

	if !validateAudioFrameParams(pcm, totalSamples, error_ptr) {
		return C.bool(false)
	}

	pcmSlice := convertPCMToSlice(pcm, totalSamples)
	err := toxavInstance.AudioSendFrame(uint32(friend_number), pcmSlice, sampleCountInt, uint8(channels), uint32(sampling_rate))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "toxav_audio_send_frame",
			"friend_number": friend_number,
			"sample_count":  sample_count,
			"channels":      channels,
			"sampling_rate": sampling_rate,
			"error":         err.Error(),
		}).Debug("Failed to send audio frame")
		mapSendFrameError(err, error_ptr)
		return C.bool(false)
	}
	return C.bool(true)
}

// validateVideoFrameParams validates video frame input parameters.
func validateVideoFrameParams(y, u, v *C.uint8_t, ySize int, error_ptr *C.TOX_AV_ERR_SEND_FRAME) bool {
	if y == nil || u == nil || v == nil || ySize <= 0 {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_NULL
		}
		return false
	}

	const maxYSize = 1 << 24 // ~16 megapixels should be enough
	if ySize > maxYSize {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_INVALID
		}
		return false
	}
	return true
}

// convertYUVToSlices converts C YUV plane data to Go slices.
func convertYUVToSlices(y, u, v *C.uint8_t, ySize, uvSize int) ([]byte, []byte, []byte) {
	ySlice := (*[1 << 24]byte)(unsafe.Pointer(y))[:ySize:ySize]
	uSlice := (*[1 << 24]byte)(unsafe.Pointer(u))[:uvSize:uvSize]
	vSlice := (*[1 << 24]byte)(unsafe.Pointer(v))[:uvSize:uvSize]
	return ySlice, uSlice, vSlice
}

// toxav_video_send_frame sends a video frame.
//
// This function matches the libtoxcore toxav_video_send_frame API exactly.
//
//export toxav_video_send_frame
func toxav_video_send_frame(av unsafe.Pointer, friend_number C.uint32_t, width, height C.uint16_t, y, u, v *C.uint8_t, error_ptr *C.TOX_AV_ERR_SEND_FRAME) C.bool {
	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_SEND_FRAME_OK
	}

	if av == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	toxavInstance, exists := toxavInstances[toxavID]
	if !exists || toxavInstance == nil {
		if error_ptr != nil {
			*error_ptr = C.TOX_AV_ERR_SEND_FRAME_SYNC
		}
		return C.bool(false)
	}

	widthInt := int(width)
	heightInt := int(height)
	ySize := widthInt * heightInt
	uvSize := ySize / 4

	if !validateVideoFrameParams(y, u, v, ySize, error_ptr) {
		return C.bool(false)
	}

	ySlice, uSlice, vSlice := convertYUVToSlices(y, u, v, ySize, uvSize)

	err := toxavInstance.VideoSendFrame(uint32(friend_number), uint16(width), uint16(height), ySlice, uSlice, vSlice)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":      "toxav_video_send_frame",
			"friend_number": friend_number,
			"width":         width,
			"height":        height,
			"error":         err.Error(),
		}).Debug("Failed to send video frame")
		mapSendFrameError(err, error_ptr)
		return C.bool(false)
	}
	return C.bool(true)
}

// Callback registration functions
// These match the libtoxcore callback registration API exactly

//export toxav_callback_call
func toxav_callback_call(av unsafe.Pointer, callback C.toxav_call_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.callCb = callback
		callbacks.callUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.callCb != nil {
				C.invoke_call_cb(
					callbacks.callCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					C.bool(audioEnabled),
					C.bool(videoEnabled),
					callbacks.callUserData,
				)
			}
		})
	}
}

//export toxav_callback_call_state
func toxav_callback_call_state(av unsafe.Pointer, callback C.toxav_call_state_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.callStateCb = callback
		callbacks.callStateUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.callStateCb != nil {
				C.invoke_call_state_cb(
					callbacks.callStateCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					C.uint32_t(state),
					callbacks.callStateUserData,
				)
			}
		})
	}
}

//export toxav_callback_audio_bit_rate
func toxav_callback_audio_bit_rate(av unsafe.Pointer, callback C.toxav_audio_bit_rate_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.audioBitRateCb = callback
		callbacks.audioBitRateUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackAudioBitRate(func(friendNumber, bitRate uint32) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.audioBitRateCb != nil {
				C.invoke_audio_bit_rate_cb(
					callbacks.audioBitRateCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					C.uint32_t(bitRate),
					callbacks.audioBitRateUserData,
				)
			}
		})
	}
}

//export toxav_callback_video_bit_rate
func toxav_callback_video_bit_rate(av unsafe.Pointer, callback C.toxav_video_bit_rate_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.videoBitRateCb = callback
		callbacks.videoBitRateUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackVideoBitRate(func(friendNumber, bitRate uint32) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.videoBitRateCb != nil {
				C.invoke_video_bit_rate_cb(
					callbacks.videoBitRateCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					C.uint32_t(bitRate),
					callbacks.videoBitRateUserData,
				)
			}
		})
	}
}

//export toxav_callback_audio_receive_frame
func toxav_callback_audio_receive_frame(av unsafe.Pointer, callback C.toxav_audio_receive_frame_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.audioReceiveFrameCb = callback
		callbacks.audioReceiveUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.audioReceiveFrameCb != nil {
				var pcmPtr *C.int16_t
				if len(pcm) > 0 {
					pcmPtr = (*C.int16_t)(unsafe.Pointer(&pcm[0]))
				}
				C.invoke_audio_receive_frame_cb(
					callbacks.audioReceiveFrameCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					pcmPtr,
					C.size_t(sampleCount),
					C.uint8_t(channels),
					C.uint32_t(samplingRate),
					callbacks.audioReceiveUserData,
				)
			}
		})
	}
}

//export toxav_callback_video_receive_frame
func toxav_callback_video_receive_frame(av unsafe.Pointer, callback C.toxav_video_receive_frame_cb, user_data unsafe.Pointer) {
	if av == nil {
		return
	}

	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return
	}

	// Store the C callback and user_data
	if callbacks, exists := toxavCallbackStorage[toxavID]; exists {
		callbacks.videoReceiveFrameCb = callback
		callbacks.videoReceiveUserData = user_data
	}

	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Capture toxavID for the closure
		capturedID := toxavID
		toxavInstance.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
			// Bridge to C callback
			toxavMutex.RLock()
			callbacks, cbExists := toxavCallbackStorage[capturedID]
			handle, handleExists := toxavHandles[capturedID]
			toxavMutex.RUnlock()

			if cbExists && handleExists && callbacks.videoReceiveFrameCb != nil {
				var yPtr, uPtr, vPtr *C.uint8_t
				if len(y) > 0 {
					yPtr = (*C.uint8_t)(unsafe.Pointer(&y[0]))
				}
				if len(u) > 0 {
					uPtr = (*C.uint8_t)(unsafe.Pointer(&u[0]))
				}
				if len(v) > 0 {
					vPtr = (*C.uint8_t)(unsafe.Pointer(&v[0]))
				}
				C.invoke_video_receive_frame_cb(
					callbacks.videoReceiveFrameCb,
					(*C.ToxAV)(handle),
					C.uint32_t(friendNumber),
					C.uint16_t(width),
					C.uint16_t(height),
					yPtr,
					uPtr,
					vPtr,
					C.int32_t(yStride),
					C.int32_t(uStride),
					C.int32_t(vStride),
					callbacks.videoReceiveUserData,
				)
			}
		})
	}
}

// Required for building as a shared library but defined in toxcore_c.go
// func main() is already defined in the main toxcore C bindings

// NOTE: This file works together with toxcore_c.go to provide
// complete ToxAV C API functionality alongside the core Tox C API.
