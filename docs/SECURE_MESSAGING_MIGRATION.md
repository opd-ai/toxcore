# Secure Messaging Migration Guide

## Summary

toxcore-go now enforces a fail-closed outbound messaging policy.
If secure encryption prerequisites are unavailable, outbound friend messages are rejected instead of being sent in plaintext.

## Behavior Change

Previous behavior:

1. Missing key provider could allow message send to proceed without encryption.
2. Message delivery could continue in plaintext compatibility mode.

Current behavior:

1. Missing encryption prerequisites blocks outbound message delivery.
2. Message state transitions to failed instead of sending plaintext.
3. Security logs include `security=plaintext_blocked` and `security=e2ee_invariant_enforced` fields.

## Why This Changed

This change enforces E2EE invariants by default and prevents accidental insecure transmission.
Protocol fallback remains capability-based (`noise+ratchet` -> `Noise-IK` -> `Legacy`) and encrypted at every level.

## Required Application Changes

1. Ensure a key provider is configured before sending friend messages.
2. Treat failed message state as a security signal when encryption setup is incomplete.
3. Add startup checks that verify key material is available for all active friend sessions.

## Operational Guidance

1. Alert on security logs containing `plaintext_blocked`.
2. Retry sends only after provisioning keys or re-establishing secure sessions.
3. Do not implement application-level plaintext fallback.

## Compatibility Notes

1. Legacy peers remain supported via encrypted legacy NaCl-box path.
2. Noise-IK peers continue to use signed negotiation and version commitment safeguards.
3. Public APIs are unchanged; behavior is hardened under the same API surface.