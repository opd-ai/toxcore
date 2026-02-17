# Audit: github.com/opd-ai/toxcore/limits
**Date**: 2026-02-17
**Status**: Complete

## Summary
The `limits` package provides centralized message size constants and validation functions for the Tox protocol. The package is well-designed with clear documentation, comprehensive table-driven tests, and proper benchmarks. No critical issues found. Test coverage is slightly below target at 60%, primarily due to two unused validation functions that should either be removed or tested.

## Issues Found
- [ ] low test-coverage — ValidateStorageMessage() has 0% coverage, unused in codebase (`limits.go:72`)
- [ ] low test-coverage — ValidateProcessingBuffer() has 0% coverage, unused in codebase (`limits.go:83`)
- [ ] low doc-coverage — Missing doc.go file for package-level documentation beyond function godocs
- [ ] low error-handling — Error messages lack context; consider structured errors with fmt.Errorf wrapping for better debugging

## Test Coverage
60.0% (target: 65%)

**Coverage Breakdown:**
- ValidateMessageSize: 100%
- ValidatePlaintextMessage: 100%
- ValidateEncryptedMessage: 100%
- ValidateStorageMessage: 0% (unused)
- ValidateProcessingBuffer: 0% (unused)

## Integration Status
The package integrates properly with the toxcore system:
- **async/storage.go**: Uses MaxPlaintextMessage and EncryptionOverhead as constants
- **crypto/constants_test.go**: Verifies MaxProcessingBuffer matches crypto layer's MaxEncryptionBuffer (1MB)
- Limited usage indicates the package is underutilized; only 2 imports found

**Integration Points:**
- ✅ Constants exported and used in async message storage
- ✅ Validation functions available but mostly unused
- ⚠️ No registration needed (pure constants/utility package)
- ⚠️ No serialization needed (stateless package)

## Recommendations
1. **Remove or test unused functions**: Either add tests for ValidateStorageMessage and ValidateProcessingBuffer (if planned for future use) or remove them to reduce code complexity and improve coverage
2. **Add package doc.go**: Create a doc.go file explaining the package's role in protocol compliance and the Tox specification relationship
3. **Consider structured errors**: Replace simple sentinel errors with wrapped errors that include context (message size, limit exceeded) for better debugging
4. **Expand usage**: The validation functions are well-designed but underutilized; consider promoting their use in messaging, file transfer, and network layers where size validation is currently done ad-hoc
