# Mobile FFI Reference Wrappers

This directory provides thin reference wrappers for consuming the toxcore C ABI
from Swift and Kotlin without requiring cgo in the mobile application.

Files:
- `swift/ToxCoreFFI.swift`: Swift wrapper around the stable C ABI symbols.
- `kotlin/ToxCoreFFI.kt`: Kotlin/JNI wrapper around the stable C ABI symbols.

These wrappers intentionally stay small and map directly to the C ABI so they
can be copied into dedicated SDK repositories with minimal changes.

## Stable ABI Contract

Consumers should use these exported C symbols at startup:
- `tox_abi_version_major`
- `tox_abi_version_minor`
- `tox_abi_version_patch`
- `tox_abi_version_string`
- `tox_abi_feature_flags`

Feature bits returned by `tox_abi_feature_flags`:
- bit 0: keypair generation (`tox_crypto_generate_keypair`)
- bit 1: secure wipe (`tox_crypto_secure_wipe`)
- bit 2: safety number (`tox_self_get_safety_number`)

Recommended startup checks:
1. Ensure ABI major version matches the SDK expectation.
2. Ensure required feature bits are present before enabling functionality.
3. Fail closed if checks do not pass.
