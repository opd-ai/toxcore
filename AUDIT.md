# C API Bindings Implementation Plan - September 6, 2025

## PROJECT STATUS

**Current State:** Core toxcore-go functionality is complete and fully tested
- ✅ Friend request protocol implemented and working
- ✅ Self information broadcasting implemented and working  
- ✅ Message validation consistency implemented and working
- ✅ All critical and medium priority bugs resolved

**Remaining Task:** Implement C API bindings as drop-in replacement for existing C toxcore

## C API BINDINGS IMPLEMENTATION PLAN

### OVERVIEW

Create a comprehensive C API layer that provides complete compatibility with the existing C toxcore library, allowing C applications to use toxcore-go as a drop-in replacement. This involves implementing CGO bindings, C header files, and ensuring API compatibility.

### PHASE 1: FOUNDATION SETUP

#### 1.1 Project Structure Setup
```
toxcore/
├── include/
│   ├── tox/
│   │   ├── tox.h              # Main C API header
│   │   ├── tox_events.h       # Event/callback definitions
│   │   └── tox_constants.h    # Constants and enums
│   └── toxcore_go.h           # Go-specific bridge header
├── capi/
│   ├── tox_capi.go           # Main CGO implementation
│   ├── events.go             # Event handling bridge
│   ├── constants.go          # Constant definitions
│   ├── memory.go             # Memory management
│   └── callbacks.go          # Callback bridging
├── examples/
│   └── c_examples/           # C usage examples
└── tests/
    └── c_integration/        # C API integration tests
```

#### 1.2 Build System Configuration
- **CMake integration** for C builds
- **pkg-config** support for library discovery
- **Cross-platform** build scripts (Linux, macOS, Windows)
- **Shared library** (.so/.dll/.dylib) generation
- **Static library** (.a/.lib) generation for embedded use

### PHASE 2: CORE API IMPLEMENTATION

#### 2.1 Instance Management (tox.h)
```c
// Core instance lifecycle
Tox *tox_new(const struct Tox_Options *options, Tox_Err_New *error);
void tox_kill(Tox *tox);
size_t tox_get_savedata_size(const Tox *tox);
void tox_get_savedata(const Tox *tox, uint8_t *savedata);

// Main loop
void tox_iterate(Tox *tox, void *user_data);
uint32_t tox_iteration_interval(const Tox *tox);
```

**Implementation Strategy:**
- CGO bridge functions that call underlying Go methods
- Instance mapping using a global registry with C pointers to Go objects
- Thread-safe access using Go mutexes
- Automatic memory management with finalizers

#### 2.2 Self Information API
```c
// Self information getters/setters
void tox_self_get_address(const Tox *tox, uint8_t *address);
void tox_self_set_nospam(Tox *tox, uint32_t nospam);
uint32_t tox_self_get_nospam(const Tox *tox);
void tox_self_get_public_key(const Tox *tox, uint8_t *public_key);
void tox_self_get_secret_key(const Tox *tox, uint8_t *secret_key);

bool tox_self_set_name(Tox *tox, const uint8_t *name, size_t length, Tox_Err_Set_Info *error);
size_t tox_self_get_name_size(const Tox *tox);
void tox_self_get_name(const Tox *tox, uint8_t *name);

bool tox_self_set_status_message(Tox *tox, const uint8_t *status_message, size_t length, Tox_Err_Set_Info *error);
size_t tox_self_get_status_message_size(const Tox *tox);
void tox_self_get_status_message(const Tox *tox, uint8_t *status_message);
```

#### 2.3 Friend Management API
```c
// Friend management
uint32_t tox_friend_add(Tox *tox, const uint8_t *address, const uint8_t *message, size_t length, Tox_Err_Friend_Add *error);
uint32_t tox_friend_add_norequest(Tox *tox, const uint8_t *public_key, Tox_Err_Friend_Add *error);
bool tox_friend_delete(Tox *tox, uint32_t friend_number, Tox_Err_Friend_Delete *error);

// Friend information
size_t tox_self_get_friend_list_size(const Tox *tox);
void tox_self_get_friend_list(const Tox *tox, uint32_t *friend_list);
bool tox_friend_exists(const Tox *tox, uint32_t friend_number);
```

#### 2.4 Messaging API
```c
// Message sending
uint32_t tox_friend_send_message(Tox *tox, uint32_t friend_number, Tox_Message_Type type, const uint8_t *message, size_t length, Tox_Err_Friend_Send_Message *error);

// Callbacks for receiving messages
typedef void tox_friend_message_cb(Tox *tox, uint32_t friend_number, Tox_Message_Type type, const uint8_t *message, size_t length, void *user_data);
void tox_callback_friend_message(Tox *tox, tox_friend_message_cb *callback);
```

#### 2.5 Network and Connection API
```c
// Bootstrap and connectivity
bool tox_bootstrap(Tox *tox, const char *address, uint16_t port, const uint8_t *public_key, Tox_Err_Bootstrap *error);
bool tox_add_tcp_relay(Tox *tox, const char *address, uint16_t port, const uint8_t *public_key, Tox_Err_Bootstrap *error);

// Connection status
Tox_Connection tox_self_get_connection_status(const Tox *tox);
typedef void tox_self_connection_status_cb(Tox *tox, Tox_Connection connection_status, void *user_data);
void tox_callback_self_connection_status(Tox *tox, tox_self_connection_status_cb *callback);
```

### PHASE 3: ADVANCED FEATURES

#### 3.1 File Transfer API
```c
// File transfer initiation
uint32_t tox_file_send(Tox *tox, uint32_t friend_number, uint32_t kind, uint64_t file_size, const uint8_t *file_id, const uint8_t *filename, size_t filename_length, Tox_Err_File_Send *error);

// File transfer control
bool tox_file_control(Tox *tox, uint32_t friend_number, uint32_t file_number, Tox_File_Control control, Tox_Err_File_Control *error);

// File transfer data
bool tox_file_send_chunk(Tox *tox, uint32_t friend_number, uint32_t file_number, uint64_t position, const uint8_t *data, size_t length, Tox_Err_File_Send_Chunk *error);
```

#### 3.2 Group Chat API
```c
// Group management
uint32_t tox_group_new(Tox *tox, Tox_Group_Privacy_State privacy_state, const uint8_t *group_name, size_t group_name_length, const uint8_t *name, size_t name_length, Tox_Err_Group_New *error);
bool tox_group_leave(Tox *tox, uint32_t group_number, const uint8_t *part_message, size_t part_message_length, Tox_Err_Group_Leave *error);

// Group messaging  
bool tox_group_send_message(Tox *tox, uint32_t group_number, Tox_Message_Type type, const uint8_t *message, size_t length, uint32_t *message_id, Tox_Err_Group_Send_Message *error);
```

### PHASE 4: MEMORY MANAGEMENT & SAFETY

#### 4.1 Memory Management Strategy
- **Reference counting** for Tox instances
- **Automatic cleanup** using Go finalizers
- **Memory pool management** for frequent allocations
- **Buffer safety checks** to prevent overflows
- **Thread-safe operations** using Go's concurrency primitives

#### 4.2 Error Handling Implementation
```c
// Error code definitions matching original toxcore
typedef enum Tox_Err_New {
    TOX_ERR_NEW_OK,
    TOX_ERR_NEW_NULL,
    TOX_ERR_NEW_MALLOC,
    TOX_ERR_NEW_PORT_ALLOC,
    TOX_ERR_NEW_PROXY_BAD_TYPE,
    TOX_ERR_NEW_PROXY_BAD_HOST,
    TOX_ERR_NEW_PROXY_BAD_PORT,
    TOX_ERR_NEW_PROXY_NOT_FOUND,
    TOX_ERR_NEW_LOAD_ENCRYPTED,
    TOX_ERR_NEW_LOAD_BAD_FORMAT,
} Tox_Err_New;
```

### PHASE 5: CALLBACK SYSTEM

#### 5.1 Callback Bridge Implementation
- **C function pointer storage** in Go callback registry
- **User data passing** through void pointers
- **Thread-safe callback invocation** from Go to C
- **Callback lifecycle management** (registration/deregistration)

#### 5.2 Event System Integration
```c
// Friend request callbacks
typedef void tox_friend_request_cb(Tox *tox, const uint8_t *public_key, const uint8_t *message, size_t length, void *user_data);
void tox_callback_friend_request(Tox *tox, tox_friend_request_cb *callback);

// Connection status callbacks
typedef void tox_friend_connection_status_cb(Tox *tox, uint32_t friend_number, Tox_Connection connection_status, void *user_data);
void tox_callback_friend_connection_status(Tox *tox, tox_friend_connection_status_cb *callback);
```

### PHASE 6: TESTING & VALIDATION

#### 6.1 Compatibility Testing
- **API signature verification** against original toxcore headers
- **Behavioral compatibility tests** for each function
- **Integration tests** with existing C applications
- **Performance benchmarks** comparing to original implementation
- **Memory leak detection** using valgrind/AddressSanitizer

#### 6.2 Documentation & Examples
- **Complete C API documentation** with doxygen
- **Migration guide** from original toxcore
- **C example applications** demonstrating all features
- **Performance characteristics** documentation
- **Building and linking instructions** for various platforms

### PHASE 7: DEPLOYMENT & DISTRIBUTION

#### 7.1 Packaging
- **Debian/Ubuntu packages** (.deb)
- **Red Hat packages** (.rpm)
- **Homebrew formula** for macOS
- **vcpkg integration** for Windows
- **Docker images** with pre-built libraries

#### 7.2 CI/CD Integration
- **Automated builds** for all supported platforms
- **Cross-compilation** testing
- **ABI compatibility checking** 
- **Performance regression testing**
- **Documentation generation** and hosting

### IMPLEMENTATION TIMELINE

**Phase 1-2 (Foundation & Core API):** 2-3 weeks
- Basic project structure and build system
- Core instance management and basic API functions
- Initial CGO bridge implementation

**Phase 3 (Advanced Features):** 2-3 weeks  
- File transfer and group chat APIs
- Complete callback system implementation
- Advanced networking features

**Phase 4-5 (Safety & Callbacks):** 1-2 weeks
- Memory management and error handling refinement
- Comprehensive callback system testing
- Thread safety validation

**Phase 6-7 (Testing & Deployment):** 2-3 weeks
- Compatibility testing and validation
- Documentation and examples
- Packaging and distribution setup

**Total Estimated Timeline:** 7-11 weeks for complete implementation

### TECHNICAL CHALLENGES

1. **Memory Management:** Bridging Go's garbage collector with C's manual memory management
2. **Callback Threading:** Ensuring callbacks execute safely across Go/C boundary
3. **ABI Compatibility:** Maintaining exact compatibility with original toxcore API
4. **Performance:** Minimizing overhead in CGO boundary crossings
5. **Platform Support:** Ensuring consistent behavior across all target platforms

### SUCCESS CRITERIA

- ✅ **100% API compatibility** with original toxcore C API
- ✅ **Drop-in replacement** capability for existing C applications
- ✅ **Performance parity** (within 10% of original implementation)
- ✅ **Memory safety** with no leaks or use-after-free issues
- ✅ **Cross-platform support** (Linux, macOS, Windows, *BSD)
- ✅ **Comprehensive test coverage** (>95% for C API layer)
- ✅ **Complete documentation** and migration guides

This implementation will provide a complete, production-ready C API that allows existing toxcore C applications to seamlessly switch to the Go implementation while maintaining full compatibility and performance.