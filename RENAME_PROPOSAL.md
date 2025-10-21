# Project Renaming Proposal

## Name Candidates

### 1. **ToxForge** — Enhanced Tox with Forward Secrecy
Highlights modernization and security enhancements while clearly maintaining Tox lineage. "Forge" suggests building upon existing foundations with improvements.

### 2. **NexTox** — Next-Generation Tox Protocol
Emphasizes evolution and future-looking improvements. Clear pronunciation and immediate understanding of Tox relationship with forward-looking stance.

### 3. **ToxPlus** — Tox Enhanced with Noise-IK Security
Simple, direct naming that signals improved version of Tox. Plus indicates additions: async messaging, Noise protocol, identity obfuscation, modern Go design.

### 4. **SecureTox** — Hardened Tox with Modern Cryptography
Security-first positioning with explicit Tox compatibility. Emphasizes the primary differentiator: Noise-IK, forward secrecy, and obfuscation features.

### 5. **ToxCore-Go** — Modern Go Implementation of Tox
Descriptive name indicating both the implementation language and protocol base. Clear technical positioning for developers seeking Go-based Tox solutions.

## Recommended Choice

**ToxForge** — This name best balances all requirements:
- Conveys enhanced security and modernization through "Forge" (crafting/strengthening)
- Maintains clear Tox ecosystem link for compatibility recognition
- Suggests active development and improvements
- Professional, memorable, and appropriate for technical audience
- Avoids potential confusion with trademark issues (descriptive/generic terms)

## Module Path and Repository

**Go Module:** `github.com/opd-ai/toxforge`  
**Repository Slug:** `toxforge`  
**Import Example:** `import "github.com/opd-ai/toxforge"`

## README Header (Rename Explanation)

**ToxForge** is a modernized, security-enhanced Go implementation compatible with the Tox messenger network. Originally based on the Tox protocol specification, ToxForge introduces significant enhancements: Noise Protocol Framework (IK pattern) for forward secrecy and KCI resistance, cryptographic peer identity obfuscation for privacy, asynchronous messaging with distributed storage, and a pure Go architecture with no CGo dependencies. While maintaining full compatibility with the Tox network for standard messaging, ToxForge extends the protocol with optional advanced features. The project acknowledges prior art from the Tox protocol specification and the Noise Protocol Framework by Trevor Perrin.

## Migration Notes

### For Existing Users
**Package Rename:** Update import paths from `github.com/opd-ai/toxcore` to `github.com/opd-ai/toxforge`. Run: `go get github.com/opd-ai/toxforge@latest` and update all imports in your code.  
**Compatibility Guarantee:** All existing Tox network connections continue to work. Your savedata, friends list, and Tox ID remain compatible. Optional features (Noise-IK, async messaging) are backward-compatible extensions.

### For Developers
**API Stability:** Core API remains unchanged. Optional advanced features (Noise transport, async manager) use explicit opt-in patterns. Standard Tox protocol operations maintain full compatibility.  
**Migration Path:** Update module references in go.mod, update import statements, rebuild. No code changes required unless adopting new features. Tests and examples updated to reflect new module path.

## Acknowledgments

ToxForge builds upon the Tox protocol specification and incorporates the Noise Protocol Framework. We acknowledge these foundational projects and their contributions to secure, decentralized communication.
