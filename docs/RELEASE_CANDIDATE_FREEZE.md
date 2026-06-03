# Release Candidate Freeze Procedure

**Purpose**: Establish a reproducible, auditable, and stable snapshot for security review, testing, and release.

**Created**: 2026-06-03  
**Scope**: Freezing toxcore-go releases for audit and public deployment

---

## Overview

A Release Candidate (RC) freeze creates an immutable snapshot of the codebase at a specific point in time. This serves multiple critical functions:

1. **Audit Reproducibility**: External auditors receive an exact commit/tag to review
2. **Deterministic Builds**: Same source always produces same binaries
3. **Testing Validation**: All test results can be linked to a specific RC tag
4. **Distribution Trust**: Users can verify the exact code they're running

---

## Release Candidate Branch Strategy

### Branch Naming Convention

```
release/v{MAJOR}.{MINOR}.{PATCH}-rc.{N}
```

**Examples**:
- `release/v2.1.0-rc.1` — First RC for v2.1.0
- `release/v2.1.0-rc.2` — Second RC (after fixes)
- `release/v3.0.0-rc.1` — Major version RC

### Timing and Creation

**When to create RC branch**:
1. All Priority 1 and Priority 2 items completed
2. Security audit plan established (scope, auditors, timeline)
3. All critical/high issues addressed
4. Full test suite green on main branch
5. Performance benchmarks acceptable

**Creation procedure**:

```bash
# 1. Ensure main is clean and up to date
git fetch origin
git checkout main
git pull origin main

# 2. Verify all tests pass
go test -race ./...
go vet ./...

# 3. Create release candidate branch
git checkout -b release/v2.1.0-rc.1

# 4. Update version strings and CHANGELOG.md
vim CHANGELOG.md  # Add release notes section
# Update version in:
# - docs/VERSION
# - go.mod (if version is tracked there)
# - README.md (if version is documented)

# 5. Commit version bump
git add CHANGELOG.md docs/VERSION README.md
git commit -m "Release candidate v2.1.0-rc.1 version bump"

# 6. Push RC branch
git push -u origin release/v2.1.0-rc.1

# 7. Create signed git tag for reproducibility
git tag -s v2.1.0-rc.1 -m "Release Candidate v2.1.0-rc.1 for security audit"

# 8. Push tag
git push origin v2.1.0-rc.1
```

---

## Release Candidate Checklist

Before freezing an RC, verify:

```yaml
pre_rc_freeze:
  code_quality:
    - [ ] go test -race ./... passes (all packages)
    - [ ] go vet ./... clean (no warnings)
    - [ ] No open TODOs in security-critical code
    - [ ] Code coverage >= 70% for security packages
  
  security:
    - [ ] All Priority 1 critical findings addressed
    - [ ] All Priority 2 implementation safeguards implemented
    - [ ] Security patch playbook in place
    - [ ] Threat model finalized
    - [ ] SECURITY.md up to date
  
  performance:
    - [ ] Baseline benchmarks recorded
    - [ ] No performance regressions > 5% from previous release
    - [ ] Memory profiling clean (no unexpected allocations)
    - [ ] Startup time acceptable
  
  documentation:
    - [ ] README.md reflects current capabilities
    - [ ] CHANGELOG.md entry for RC
    - [ ] Security docs synchronized with implementation
    - [ ] API documentation current
    - [ ] Migration guide ready (if breaking changes)
  
  testing:
    - [ ] Unit tests all passing
    - [ ] Integration tests all passing
    - [ ] Compatibility tests passing (Legacy, Noise-IK, Noise+Ratchet)
    - [ ] Protocol tests passing (negotiation, fallback, MITM detection)
  
  build_and_distribution:
    - [ ] Build reproducible on CI/CD
    - [ ] Cross-platform builds successful (Linux, macOS, Windows)
    - [ ] Docker image (if applicable) builds successfully
    - [ ] Release artifacts (binaries, checksums) staged
```

---

## Audit Snapshot Delivery

### What to Provide to Auditors

**1. Source Code**:
```bash
# Auditors receive:
- Git repository clone (or tarball of specific tag)
- Specific tag: v2.1.0-rc.1
- Clone command:
  git clone --depth=1 --branch v2.1.0-rc.1 \
    https://github.com/opd-ai/toxcore.git
```

**2. Build Instructions**:
```bash
# File: AUDIT_BUILD.md
## Building the Audit Snapshot

### Prerequisites
- Go 1.25+
- (Other dependencies listed in go.mod)

### Build Steps
1. Clone: git clone --branch v2.1.0-rc.1 ...
2. Verify: go mod verify
3. Build: go build ./...
4. Test: go test -race ./...
5. Binary location: ./cmd/toxcore (or location specific to project)

### Verify Binary Integrity
# Compare checksums (provided in release notes)
sha256sum ./toxcore
# Expected: [SHA256 provided in release notes]
```

**3. Threat Model & Audit Scope**:
- File: `docs/THREAT_MODEL.md`
- File: `docs/AUDIT_SCOPE.md`
- Specifies what auditors should review

**4. Code Map**:
- File: `docs/CODE_MAP.md` — High-level package structure
- Example:
  ```
  ./crypto/         — Cryptographic operations (Curve25519, Noise, Ratchet)
  ./noise/          — Noise protocol implementation
  ./ratchet/        — Post-compromise security ratchet
  ./async/          — Asynchronous messaging and offline delivery
  ./transport/      — UDP/TCP packet transport
  ./messaging/      — Message routing and delivery
  ./dht/            — Distributed hash table (peer discovery)
  ```

**5. Known Issues/Deferred Items**:
- File: `docs/KNOWN_ISSUES.md`
- Lists any known limitations, planned future work, or explicitly deferred findings

---

## RC to Final Release Workflow

### Scenario: No Issues Found

```bash
# 1. After audit clearance, create release tag
git tag -s v2.1.0 -m "Release v2.1.0"

# 2. Build and publish binaries
# (CI/CD handles this automatically on tag push)

# 3. Create GitHub release
gh release create v2.1.0 \
  --notes "See CHANGELOG.md for details" \
  --draft=false

# 4. Announce release
# (Send security advisory if needed, post release notes)
```

### Scenario: Issues Found, RC2 Needed

```bash
# 1. Switch back to main
git checkout main
git pull origin main

# 2. Fix the issue(s) on main
# (Code changes, PR, review, merge)

# 3. Create new RC branch
git checkout -b release/v2.1.0-rc.2

# 4. Update version and CHANGELOG
vim CHANGELOG.md  # Add RC.2 entry
git commit -am "RC2: Fix [specific issue]"

# 5. Push and tag
git push -u origin release/v2.1.0-rc.2
git tag -s v2.1.0-rc.2 -m "Release Candidate v2.1.0-rc.2"
git push origin v2.1.0-rc.2

# 6. Notify auditors of new snapshot
# Include change summary and impact assessment
```

---

## CI/CD Integration

### GitHub Actions for RC Workflow

**File**: `.github/workflows/release-candidate.yml`

```yaml
name: Release Candidate Management

on:
  push:
    branches:
      - release/*-rc.*

jobs:
  verify_rc:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Verify RC is clean
        run: |
          go mod verify
          go vet ./...
          go test -race ./...
      
      - name: Build RC artifacts
        run: |
          go build -o ./toxcore ./cmd/toxcore
          sha256sum ./toxcore > toxcore.sha256
      
      - name: Upload RC artifacts
        uses: actions/upload-artifact@v4
        with:
          name: rc-artifacts
          path: |
            ./toxcore
            ./toxcore.sha256
      
      - name: Create RC GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: ${{ github.ref_name }}
          draft: true
          prerelease: true
          files: |
            ./toxcore
            ./toxcore.sha256
          body: |
            Release Candidate: ${{ github.ref_name }}
            
            **Important**: This is a release candidate for security audit.
            Do not deploy to production without final release.
            
            Build: ${{ github.sha }}
            Workflow: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
```

---

## RC Lifecycle and Retention

### Policy

```
RC Retention:
  - Keep all RC tags in git indefinitely (immutable audit trail)
  - RC binaries stored for 30 days after final release
  - Delete RC branches 30 days after final release created
  - Keep CHANGELOG.md entries for all RC attempts
```

### Example Timeline

```
Day 1:  Create release/v2.1.0-rc.1 tag
Days 2-7: Audit in progress
Day 8:  Issue found, RC1 archived, RC2 created
Days 9-14: Audit resumes
Day 15: Audit complete, v2.1.0 final tag created
Day 16: Release published, binaries distributed
Day 46: RC branches cleaned up (30 days later)
Forever: Git tags and history preserved
```

---

## Reproducible Build Verification

### Build Reproducibility Check

After creating RC, verify that the same source produces the same binary:

```bash
# 1. Build from RC tag on machine A
git checkout v2.1.0-rc.1
go build -o /tmp/toxcore_a ./cmd/toxcore

# 2. Build from RC tag on machine B (different environment)
git checkout v2.1.0-rc.1
go build -o /tmp/toxcore_b ./cmd/toxcore

# 3. Verify checksums match
sha256sum /tmp/toxcore_a /tmp/toxcore_b
# Both should show identical SHA256

# 4. If checksums differ, investigate:
# - Go version differences
# - GOFLAGS environment variable
# - Time-based build metadata
# - Compiler version
```

### Deterministic Build Requirements

Ensure builds are deterministic by:

```bash
# 1. Pin Go version in go.mod
go mod edit -go=1.25

# 2. Lock all dependencies
go mod tidy
git add go.mod go.sum
git commit -m "Lock dependencies for reproducible builds"

# 3. Disable build time embedding (if used)
# Use git commit hash instead of time-based tags:
go build -ldflags="-X main.Commit=$(git rev-parse HEAD)" ./cmd/toxcore
```

---

## Audit Completion and Sign-off

### Final Sign-off Template

Once audit is complete, create a formal record:

**File**: `docs/AUDIT_SIGN_OFF_v2.1.0.md`

```markdown
# Audit Sign-Off: v2.1.0

## Release Candidate
- Tag: v2.1.0-rc.1 (final RC reviewed)
- Build Date: [DATE]
- Build Commit: [COMMIT SHA]

## Audit Details
- **Auditor**: [Audit Firm/Individual]
- **Audit Period**: [START] to [END]
- **Scope**: [Reference AUDIT_SCOPE.md]

## Findings Summary
- **Critical**: 0 (0 unresolved)
- **High**: 0 (0 unresolved)
- **Medium**: 2 (2 resolved in follow-up work)
- **Low**: 3 (1 deferred, 2 resolved)

## Findings by Category
1. [Link to findings]
2. [Status of each finding]

## Auditor Recommendation
✅ APPROVED FOR RELEASE

Signature: ________________________________  Date: ___________

## Release Publication
- **Released**: [DATE]
- **Final Tag**: v2.1.0
- **Distribution**: GitHub Releases, pkg.go.dev
```

---

## References

- [Reproducible Builds](https://reproducible-builds.org/)
- [Go Module Documentation](https://go.dev/ref/mod)
- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github)
- [Security Audits Best Practices](https://csrc.nist.gov/publications/detail/sp/800-161/final)
