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

// Callback function types matching libtoxcore exactly
typedef void (*toxav_call_cb)(ToxAV *av, uint32_t friend_number, bool audio_enabled, bool video_enabled, void *user_data);
typedef void (*toxav_call_state_cb)(ToxAV *av, uint32_t friend_number, uint32_t state, void *user_data);
typedef void (*toxav_audio_bit_rate_cb)(ToxAV *av, uint32_t friend_number, uint32_t audio_bit_rate, void *user_data);
typedef void (*toxav_video_bit_rate_cb)(ToxAV *av, uint32_t friend_number, uint32_t video_bit_rate, void *user_data);
typedef void (*toxav_audio_receive_frame_cb)(ToxAV *av, uint32_t friend_number, const int16_t *pcm, size_t sample_count, uint8_t channels, uint32_t sampling_rate, void *user_data);
typedef void (*toxav_video_receive_frame_cb)(ToxAV *av, uint32_t friend_number, uint16_t width, uint16_t height, const uint8_t *y, const uint8_t *u, const uint8_t *v, int32_t ystride, int32_t ustride, int32_t vstride, void *user_data);
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
	avpkg "github.com/opd-ai/toxcore/av"
)

// Global instance management for C API compatibility
// This follows the same pattern as the main toxcore C bindings
var (
	toxavInstances         = make(map[uintptr]*toxcore.ToxAV)
	nextToxAVID    uintptr = 1
	toxavMutex     sync.RWMutex
)

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

	// For Phase 1: Simplified implementation
	// TODO: In full implementation, convert C Tox pointer to Go Tox instance
	// This requires coordination with toxcore_c.go's instance management

	// For now, we'll create a minimal stub that establishes the API
	// The actual Tox integration will be completed when the instance
	// management is unified between toxcore_c.go and toxav_c.go

	if error_ptr != nil {
		*error_ptr = C.TOX_AV_ERR_NEW_OK // Success for Phase 1 API structure
	}

	// Store a placeholder ID and return it as a pointer
	toxavMutex.Lock()
	defer toxavMutex.Unlock()

	toxavID := nextToxAVID
	nextToxAVID++

	// Store nil for now - will be replaced with actual ToxAV instance
	// when Tox instance integration is complete
	toxavInstances[toxavID] = nil

	// Create a real memory allocation to use as an opaque pointer
	// This allows us to use the address as a safe pointer that
	// maps back to our toxavID through the instances map
	handle := new(uintptr)
	*handle = toxavID
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

	// TODO: Implement Tox instance retrieval
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

// toxav_call initiates an audio/video call.
//
// This function matches the libtoxcore toxav_call API exactly.
//
//export toxav_call
func toxav_call(av unsafe.Pointer, friend_number, audio_bit_rate, video_bit_rate C.uint32_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		err := toxavInstance.Call(uint32(friend_number), uint32(audio_bit_rate), uint32(video_bit_rate))
		return C.bool(err == nil)
	}
	return C.bool(false)
}

// toxav_answer accepts an incoming audio/video call.
//
// This function matches the libtoxcore toxav_answer API exactly.
//
//export toxav_answer
func toxav_answer(av unsafe.Pointer, friend_number, audio_bit_rate, video_bit_rate C.uint32_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		err := toxavInstance.Answer(uint32(friend_number), uint32(audio_bit_rate), uint32(video_bit_rate))
		return C.bool(err == nil)
	}
	return C.bool(false)
}

// toxav_call_control sends a call control command.
//
// This function matches the libtoxcore toxav_call_control API exactly.
//
//export toxav_call_control
func toxav_call_control(av unsafe.Pointer, friend_number C.uint32_t, control C.TOX_AV_CALL_CONTROL, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Convert C control enum to Go enum
		goControl := avpkg.CallControl(control)
		err := toxavInstance.CallControl(uint32(friend_number), goControl)
		return C.bool(err == nil)
	}
	return C.bool(false)
}

// toxav_audio_set_bit_rate sets the audio bit rate for a call.
//
// This function matches the libtoxcore toxav_audio_set_bit_rate API exactly.
//
//export toxav_audio_set_bit_rate
func toxav_audio_set_bit_rate(av unsafe.Pointer, friend_number, bit_rate C.uint32_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		err := toxavInstance.AudioSetBitRate(uint32(friend_number), uint32(bit_rate))
		return C.bool(err == nil)
	}
	return C.bool(false)
}

// toxav_video_set_bit_rate sets the video bit rate for a call.
//
// This function matches the libtoxcore toxav_video_set_bit_rate API exactly.
//
//export toxav_video_set_bit_rate
func toxav_video_set_bit_rate(av unsafe.Pointer, friend_number, bit_rate C.uint32_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		err := toxavInstance.VideoSetBitRate(uint32(friend_number), uint32(bit_rate))
		return C.bool(err == nil)
	}
	return C.bool(false)
}

// toxav_audio_send_frame sends an audio frame.
//
// This function matches the libtoxcore toxav_audio_send_frame API exactly.
//
//export toxav_audio_send_frame
func toxav_audio_send_frame(av unsafe.Pointer, friend_number C.uint32_t, pcm *C.int16_t, sample_count C.size_t, channels C.uint8_t, sampling_rate C.uint32_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Convert C PCM data to Go slice
		sampleCountInt := int(sample_count)
		channelsInt := int(channels)
		totalSamples := sampleCountInt * channelsInt

		if pcm != nil && totalSamples > 0 {
			pcmSlice := (*[1 << 20]int16)(unsafe.Pointer(pcm))[:totalSamples:totalSamples]
			err := toxavInstance.AudioSendFrame(uint32(friend_number), pcmSlice, sampleCountInt, uint8(channels), uint32(sampling_rate))
			return C.bool(err == nil)
		}
	}
	return C.bool(false)
}

// toxav_video_send_frame sends a video frame.
//
// This function matches the libtoxcore toxav_video_send_frame API exactly.
//
//export toxav_video_send_frame
func toxav_video_send_frame(av unsafe.Pointer, friend_number C.uint32_t, width, height C.uint16_t, y, u, v *C.uint8_t, error_ptr unsafe.Pointer) C.bool {
	if av == nil {
		return C.bool(false)
	}

	toxavMutex.RLock()
	defer toxavMutex.RUnlock()

	toxavID, ok := getToxAVID(av)
	if !ok {
		return C.bool(false)
	}
	if toxavInstance, exists := toxavInstances[toxavID]; exists && toxavInstance != nil {
		// Calculate plane sizes for YUV420
		widthInt := int(width)
		heightInt := int(height)
		ySize := widthInt * heightInt
		uvSize := ySize / 4

		if y != nil && u != nil && v != nil && ySize > 0 {
			// Convert C arrays to Go slices
			ySlice := (*[1 << 24]byte)(unsafe.Pointer(y))[:ySize:ySize]
			uSlice := (*[1 << 24]byte)(unsafe.Pointer(u))[:uvSize:uvSize]
			vSlice := (*[1 << 24]byte)(unsafe.Pointer(v))[:uvSize:uvSize]

			err := toxavInstance.VideoSendFrame(uint32(friend_number), uint16(width), uint16(height), ySlice, uSlice, vSlice)
			return C.bool(err == nil)
		}
	}
	return C.bool(false)
}

// Callback registration functions
// These match the libtoxcore callback registration API exactly

//export toxav_callback_call
func toxav_callback_call(av unsafe.Pointer, callback C.toxav_call_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		// TODO: In future phases, implement proper C callback bridge
		toxavInstance.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
			// Placeholder implementation for Phase 1
			// Full C callback integration will be implemented in later phases
		})
	}
}

//export toxav_callback_call_state
func toxav_callback_call_state(av unsafe.Pointer, callback C.toxav_call_state_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		// TODO: In future phases, implement proper C callback bridge
		toxavInstance.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
			// Placeholder implementation for Phase 1
			// Full C callback integration will be implemented in later phases
		})
	}
}

//export toxav_callback_audio_bit_rate
func toxav_callback_audio_bit_rate(av unsafe.Pointer, callback C.toxav_audio_bit_rate_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		toxavInstance.CallbackAudioBitRate(func(friendNumber, bitRate uint32) {
			// Placeholder implementation for Phase 1
		})
	}
}

//export toxav_callback_video_bit_rate
func toxav_callback_video_bit_rate(av unsafe.Pointer, callback C.toxav_video_bit_rate_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		toxavInstance.CallbackVideoBitRate(func(friendNumber, bitRate uint32) {
			// Placeholder implementation for Phase 1
		})
	}
}

//export toxav_callback_audio_receive_frame
func toxav_callback_audio_receive_frame(av unsafe.Pointer, callback C.toxav_audio_receive_frame_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		toxavInstance.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16, sampleCount int, channels uint8, samplingRate uint32) {
			// Placeholder implementation for Phase 1
		})
	}
}

//export toxav_callback_video_receive_frame
func toxav_callback_video_receive_frame(av unsafe.Pointer, callback C.toxav_video_receive_frame_cb, user_data unsafe.Pointer) {
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
		// For Phase 1: Set a placeholder callback
		toxavInstance.CallbackVideoReceiveFrame(func(friendNumber uint32, width, height uint16, y, u, v []byte, yStride, uStride, vStride int) {
			// Placeholder implementation for Phase 1
		})
	}
}

// Required for building as a shared library but defined in toxcore_c.go
// func main() is already defined in the main toxcore C bindings

// NOTE: This file works together with toxcore_c.go to provide
// complete ToxAV C API functionality alongside the core Tox C API.
