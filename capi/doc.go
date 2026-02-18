// Package main provides C API bindings for toxcore-go, enabling cross-language
// interoperability with C applications and other language bindings.
//
// # Overview
//
// The capi package implements a C-compatible API that matches the libtoxcore
// interface exactly, enabling seamless integration with existing C applications.
// This package provides bindings for both the core Tox API (toxcore_c.go) and
// the ToxAV audio/video API (toxav_c.go).
//
// # Build Instructions
//
// To build as a C shared library:
//
//	go build -buildmode=c-shared -o libtoxcore.so ./capi/
//
// This generates:
//   - libtoxcore.so: The shared library
//   - libtoxcore.h: Auto-generated C header file with function declarations
//
// # C API Usage
//
// The C API follows the same patterns as the original libtoxcore:
//
//	#include "libtoxcore.h"
//
//	// Create a new Tox instance
//	Tox *tox = tox_new(NULL, NULL);
//	if (tox == NULL) {
//	    fprintf(stderr, "Failed to create Tox instance\n");
//	    return 1;
//	}
//
//	// Bootstrap to the network
//	const char *bootstrap_key = "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67";
//	tox_bootstrap(tox, "node.tox.biribiri.org", 33445, bootstrap_key, NULL);
//
//	// Main loop
//	while (running) {
//	    tox_iterate(tox, NULL);
//	    usleep(tox_iteration_interval(tox) * 1000);
//	}
//
//	// Cleanup
//	tox_kill(tox);
//
// # ToxAV API Usage
//
// For audio/video functionality:
//
//	// Create ToxAV from existing Tox instance
//	ToxAV *toxav = toxav_new(tox, NULL);
//
//	// Register callbacks
//	toxav_callback_call(toxav, on_call, user_data);
//	toxav_callback_audio_receive_frame(toxav, on_audio_frame, user_data);
//
//	// Iterate alongside main Tox loop
//	toxav_iterate(toxav);
//
//	// Cleanup
//	toxav_kill(toxav);
//
// # Callback Bridging
//
// The C API properly bridges C function pointers to Go callbacks. When you
// register a C callback function, it will be invoked from Go when the
// corresponding event occurs. The user_data pointer is preserved and passed
// through to your callback.
//
// Important: C callbacks are invoked from Go goroutines. Ensure your C code
// is thread-safe or uses appropriate synchronization.
//
// # Thread Safety
//
// The C API is thread-safe. Internal mutex protection ensures that concurrent
// calls from multiple threads are properly synchronized. However, callback
// invocations may occur on different threads than the original API calls.
//
// # Instance Management
//
// The C API uses opaque pointers (handles) to represent Tox and ToxAV instances.
// These handles are managed internally and map to Go objects. Always use
// tox_kill() and toxav_kill() to properly release resources.
//
// # Error Handling
//
// Functions that can fail provide error output parameters (similar to libtoxcore):
//
//	TOX_ERR_NEW err;
//	Tox *tox = tox_new(NULL, &err);
//	if (err != TOX_ERR_NEW_OK) {
//	    // Handle error
//	}
//
// Boolean return values indicate success (true) or failure (false) for
// operations that don't have dedicated error enums.
//
// # Limitations
//
//   - The package must be built as "package main" with a main() function
//     to work as a c-shared library
//   - Some advanced Go features (like context cancellation) are not exposed
//     through the C API
//   - Memory management follows C conventions - the caller is responsible
//     for freeing any allocated memory from output parameters
//
// # Files
//
//   - toxcore_c.go: Core Tox C API functions
//   - toxav_c.go: ToxAV audio/video C API functions
//   - doc.go: This documentation file
package main
