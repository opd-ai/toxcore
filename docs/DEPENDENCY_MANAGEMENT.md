# Dependency Security and Risk Management

**Date**: 2026-06-03  
**Purpose**: Define processes for tracking, auditing, and managing dependencies securely

## Table of Contents

1. [Overview](#overview)
2. [Dependency Inventory](#dependency-inventory)
3. [Risk Assessment](#risk-assessment)
4. [Security Scanning](#security-scanning)
5. [Update Strategy](#update-strategy)
6. [Lockfile Verification](#lockfile-verification)
7. [Incident Response](#incident-response)
8. [Automation & Tooling](#automation--tooling)

---

## Overview

toxcore-go maintains a minimal set of dependencies to reduce attack surface and maintenance burden. All dependencies are assessed for:

1. **Security**: Known vulnerabilities, maintenance status
2. **License**: Compatibility with MIT license (project license)
3. **Supply Chain**: Provenance, maintainer reputation
4. **Maintenance**: Activity level, responsiveness to issues
5. **Bloat**: Size, number of transitive dependencies

---

## Dependency Inventory

### Current Dependencies (as of 2026-06-03)

#### Direct Dependencies

| Module | Purpose | Risk Level | License | Last Update |
|--------|---------|-----------|---------|-------------|
| `github.com/flynn/noise` | Noise protocol | Medium | BSD-2 | v1.1.0 |
| `github.com/go-i2p/onramp` | I2P connectivity | Medium | MIT | v0.33.92 |
| `github.com/klauspost/reedsolomon` | Reed-Solomon erasure coding | Low | MIT | v1.13.3 |
| `github.com/opd-ai/magnum` | Opus audio codec | Low | MIT | v0.0.0-20260324142352-b5664a8a5c6a |
| `github.com/opd-ai/vp8` | VP8 video codec | Low | Apache 2.0 | v0.0.0-20260407023446-a01cf06c95d4 |
| `github.com/pion/rtp` | RTP packet support | Low | MIT | v1.8.22 |
| `github.com/sirupsen/logrus` | Logging | Low | MIT | v1.9.4 |
| `github.com/stretchr/testify` | Testing assertions | Low | MIT | v1.11.1 |
| `github.com/xlab/libvpx-go` | VP8 bindings | Medium | BSD-3 | v0.0.0-20220203233824-652b2616315c |
| `golang.org/x/crypto` | Curve25519, Ed25519, ChaCha20 | Low | BSD-3 | v0.48.0 |
| `golang.org/x/image` | Image and colorspace handling | Low | BSD-3 | v0.38.0 |
| `golang.org/x/net` | IPv6 and network helpers | Low | BSD-3 | v0.50.0 |
| `golang.org/x/sync` | Concurrency primitives | Low | BSD-3 | v0.8.0 |
| `golang.org/x/sys` | OS interfaces | Low | BSD-3 | v0.41.0 |

#### Transitive Dependencies (Critical Only)

| Module | Required By | Risk | Purpose |
|--------|------------|------|---------|
| `golang.org/x/sys` | x/crypto, x/net | Low | OS interfaces |
| `golang.org/x/text` | x/net | Low | Unicode handling |

### Dependency Justification

- **x/crypto**: Standard library crypto implementations; maintained by Go team
- **x/net**: Essential for IPv6 and TLS; maintained by Go team
- **magnum, vp8**: Custom opd-ai implementations for audio/video codec
- **flynn/noise**: Vetted Noise Protocol implementation (CVE-2021-4239 patched)
- **logrus**: Widely used, well-maintained structured logging

---

## Risk Assessment

### Assessment Framework

Each dependency is evaluated quarterly using:

1. **Vulnerability History** (0-10 points, lower is better)
   - 0: No known CVEs
   - 5: 1-2 historical CVEs, all patched
   - 10: Current unpatched CVEs

2. **Maintenance Activity** (0-10 points, lower is better)
   - 0: Regular updates, responsive maintainers
   - 5: Slow updates, but stable
   - 10: Abandoned or infrequent updates

3. **Supply Chain Risk** (0-10 points, lower is better)
   - 0: Single maintainer, verified commits
   - 5: Small team, some activity
   - 10: Unknown/untrusted source, no verification

4. **Scope Creep** (0-10 points, lower is better)
   - 0: Focused, minimal transitive deps
   - 5: Some bloat, but acceptable
   - 10: Heavy dependencies, high transitive load

**Risk Score Calculation**: (Vulnerability + Maintenance + Supply Chain + Scope) / 4

**Risk Categories**:
- **Green** (Score < 3): Low risk, safe to use
- **Yellow** (Score 3-6): Medium risk, monitor
- **Red** (Score > 6): High risk, consider alternatives

### Last Assessment Results (2026-06-03)

| Module | Vuln | Maint | Supply | Scope | **Score** | **Status** |
|--------|------|-------|--------|-------|-----------|-----------|
| x/crypto | 0 | 0 | 0 | 0 | **0.0** | 🟢 Green |
| x/net | 0 | 0 | 0 | 0 | **0.0** | 🟢 Green |
| magnum | 0 | 2 | 2 | 1 | **1.25** | 🟢 Green |
| vp8 | 0 | 2 | 2 | 1 | **1.25** | 🟢 Green |
| noise | 1 | 1 | 1 | 1 | **1.0** | 🟢 Green |
| logrus | 0 | 2 | 0 | 2 | **1.0** | 🟢 Green |

**Summary**: All dependencies rated GREEN. No immediate action required.

---

## Security Scanning

### Automated Scanning Tools

#### 1. Govulncheck (Official Go Vulnerability Database)

**Tool**: `golang.org/x/vuln/cmd/govulncheck`

**Schedule**: On every commit (CI) + daily

**Command**:
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

**Integration**: GitHub Actions in `toxcore.yml`

**Failure Policy**: 
- Current CVEs: Block release
- Historical CVEs (patched): Warn only

#### 2. Dependency Check (Broader Coverage)

**Tool**: OWASP Dependency-Check

**Schedule**: Weekly

**Command**:
```bash
docker run --rm -v $(pwd):/src \
  owasp/dependency-check:latest \
  --project toxcore-go \
  --scan /src \
  --format HTML
```

**Report Location**: `.github/dependency-check-report.html`

**Action**: Review for non-Go dependency issues

#### 3. SBOMgo (Software Bill of Materials)

**Tool**: Generate SBOM for release transparency

**Schedule**: On version tags

**Command**:
```bash
go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
cyclonedx-gomod app -output sbom.json
```

**Artifact**: Included in GitHub release for supply chain verification

---

## Update Strategy

### Patch Updates (Patch Version Bump)

**Trigger**: Security fix in dependency, no breaking changes

**Procedure**:
```bash
# 1. Update specific module
go get -u=patch github.com/org/module

# 2. Verify no breaking changes
go mod tidy
go build ./...
go test -race ./...

# 3. Verify security fix
govulncheck ./...

# 4. Commit and release
git add go.mod go.sum
git commit -m "chore: bump [module] for security patch"
git tag -s vX.Y.(Z+1)
```

**Example**: v2.1.2 → v2.1.3

### Minor Updates (Minor Version Bump)

**Trigger**: New features in dependency, no breaking changes

**Procedure**:
1. Update module: `go get -u=minor github.com/org/module`
2. Review changelog for impact on toxcore-go
3. Update docs if new features are relevant
4. Run full test suite
5. Release as minor version bump

**Cadence**: Every 2-4 weeks (batch multiple updates)

**Example**: v2.1.2 → v2.2.0

### Major Updates (Major Version Bump)

**Trigger**: Breaking changes or major refactoring

**Procedure**:
1. **Research Phase** (1-2 weeks)
   - Review migration guide
   - Identify impact on toxcore-go code
   - Plan code changes

2. **Development Phase** (2-4 weeks)
   - Update to new version
   - Refactor affected code
   - Add integration tests
   - Update documentation

3. **Testing Phase** (1 week)
   - Full regression test suite
   - Performance benchmarks
   - Protocol compatibility matrix
   - Security scan

4. **Release Phase**
   - Major version bump
   - Detailed release notes
   - Migration guide for users

**Example**: v2.1.2 → v3.0.0

### Dependency Update Checklist

```yaml
dependency_update:
  pre_update:
    - [ ] Review dependency changelog
    - [ ] Check for breaking changes
    - [ ] Assess security impact
    - [ ] Plan code changes (if needed)
  
  update:
    - [ ] Run `go get -u[flags] module`
    - [ ] Run `go mod tidy`
    - [ ] Run `go build ./...`
    - [ ] Review go.sum changes (new entries?)
  
  verify:
    - [ ] `go vet ./...` passes
    - [ ] `go test -race ./...` passes
    - [ ] `govulncheck ./...` clean
    - [ ] `go-stats-generator diff` shows no regressions
  
  test:
    - [ ] Unit tests pass
    - [ ] Integration tests pass
    - [ ] Benchmark results acceptable
    - [ ] Protocol compatibility tests pass
  
  commit:
    - [ ] Single atomic commit
    - [ ] Clear commit message
    - [ ] Update CHANGELOG.md
    - [ ] Update docs if needed
```

---

## Lockfile Verification

### go.sum Integrity

The `go.sum` file records cryptographic hashes of all dependencies. Always verify:

#### 1. Hash Verification (Automatic)

Go automatically verifies hashes on `go mod tidy`:

```bash
go mod verify
# Output: all modules verified
```

**What it checks**:
- Downloaded modules match recorded hashes
- No tampering or corruption
- Consistent across builds

#### 2. Manual Verification

```bash
# Download and manually verify a module
go get -verify=mod github.com/org/module@version

# Check specific module hash
go mod download -json github.com/org/module@version | \
  jq .Hash
```

#### 3. go.sum Audit

**Script** (`scripts/audit-lockfile.sh`):

```bash
#!/bin/bash
set -e

echo "🔒 Auditing go.sum integrity..."

# Verify all modules
go mod verify

# Check for indirect unexplained modules
echo "📊 Analyzing dependency tree..."
go mod graph | sort | uniq > /tmp/deps.txt
echo "$(wc -l < /tmp/deps.txt) dependencies found"

# Check for duplicate entries
duplicates=$(awk '{print $1}' go.sum | sort | uniq -d)
if [ -n "$duplicates" ]; then
    echo "⚠️  WARNING: Duplicate entries in go.sum:"
    echo "$duplicates"
    exit 1
fi

# Check for suspicious entries (from typosquatters, etc.)
echo "✅ go.sum integrity verified"
```

**Run Before Each Release**:

```bash
bash scripts/audit-lockfile.sh
```

#### 4. Supply Chain Security Checks

```bash
# 1. Verify module authenticity
for module in $(go list -m all); do
    echo "Checking $module..."
    go mod download -verify=mod "$module" || exit 1
done

# 2. Check for modules with no go.mod
go mod graph | grep -v "go.mod" || echo "All modules have go.mod"

# 3. Validate against known registries
# (Ensure dependencies come from official sources)
go env GOPROXY
```

---

## Periodic Review Schedule

### Weekly (Automated)

- [ ] Run govulncheck in CI
- [ ] Monitor GitHub Dependabot alerts
- [ ] Review new CVE disclosures

### Monthly (Manual Review)

- [ ] Review dependency update PRs
- [ ] Check dependency activity (commits, releases)
- [ ] Audit go.sum for unexpected changes
- [ ] Update DEPENDENCY_REPORT.md

### Quarterly (Full Assessment)

- [ ] Risk assessment for all dependencies
- [ ] Review alternative/replacement packages
- [ ] Plan major version updates
- [ ] Update this document

### Annually (Strategic Review)

- [ ] Evaluate overall dependency strategy
- [ ] Consider removing unnecessary dependencies
- [ ] Assess build/test infrastructure changes
- [ ] Plan multi-version support

---

## Incident Response

### Dependency Vulnerability Discovered

**Severity: Critical** (CVSS ≥ 9.0)

```
Immediate Actions (< 4 hours):
  1. Assess impact on toxcore-go
  2. Determine if vulnerability is exploitable in our context
  3. If exploitable:
     - Begin patch development immediately
     - Update to patched dependency version
     - Run full test suite
     - Release patch within 24 hours
  4. If not exploitable:
     - Document why and monitor for exploitation
     - Update at next scheduled release if available
```

**Severity: High** (CVSS 7-8.9)

```
Standard Actions (< 72 hours):
  1. Assess impact
  2. Schedule update to patched version
  3. Include in next release cycle
  4. Notify users if urgency warranted
```

**Severity: Medium/Low** (CVSS < 7)

```
Regular Cycle (< 30 days):
  1. Include in next scheduled update
  2. Batch with other updates if possible
  3. Include in release notes
```

### Dependency Abandoned

**Indicators**:
- No commits in 2+ years
- Unresponded security issues
- Incompatibility with new Go versions

**Response**:
1. **Immediate**: Create contingency plan
2. **30 Days**: Evaluate replacements
3. **60 Days**: Fork if critical, or find alternative
4. **Release**: Update documentation

### Supply Chain Attack Detection

**Suspicious Indicators**:
- Unusual commit activity spike
- New maintainer with extensive permissions
- Significant code changes by new contributor
- Build system changes

**Verification Steps**:

```bash
# 1. Check module authenticity
go mod download -json module@version | jq '.

# 2. Verify GPG signatures (if available)
git clone --depth=1 https://github.com/org/repo
cd repo && git log --verify-signatures | head -20

# 3. Compare with known-good hashes
# (Maintain out-of-band verification of critical modules)
```

---

## Automation & Tooling

### CI Integration

**File**: `.github/workflows/dependency-check.yml`

```yaml
name: Dependency Security Check

on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM UTC

jobs:
  govulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: govulncheck ./...
  
  mod-verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go mod verify
```

### Local Pre-Commit Hook

**File**: `.git/hooks/pre-commit`

```bash
#!/bin/bash
set -e

echo "🔐 Running dependency verification..."
go mod verify || exit 1
go vet ./... || exit 1

echo "✅ Dependencies verified, commit allowed"
```

**Installation**:

```bash
cp scripts/pre-commit-hook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

### Monthly Dependency Report

**Script**: `scripts/generate-dependency-report.sh`

```bash
#!/bin/bash

echo "# Monthly Dependency Report - $(date +%Y-%m-%d)" > DEPENDENCY_REPORT.md
echo "" >> DEPENDENCY_REPORT.md

echo "## Direct Dependencies" >> DEPENDENCY_REPORT.md
go list -m all >> DEPENDENCY_REPORT.md

echo "" >> DEPENDENCY_REPORT.md
echo "## Security Status" >> DEPENDENCY_REPORT.md
govulncheck -json ./... | jq .modules >> DEPENDENCY_REPORT.md
```

---

## Best Practices

### Do's

- ✅ **Do** pin Go version in go.mod (using `go` directive)
- ✅ **Do** run `go mod tidy` before committing
- ✅ **Do** review `go.sum` diffs in PRs
- ✅ **Do** verify modules with `go mod verify`
- ✅ **Do** keep dependencies as few and stable as possible
- ✅ **Do** test all updates thoroughly
- ✅ **Do** document dependency choices in comments

### Don'ts

- ❌ **Don't** manually edit `go.sum` (use `go mod`)
- ❌ **Don't** ignore govulncheck warnings
- ❌ **Don't** update dependencies without testing
- ❌ **Don't** use unreleased versions (`main` branch)
- ❌ **Don't** depend on deprecated packages
- ❌ **Don't** commit local replacements (`replace` directives)

---

## References

- [Go Modules](https://go.dev/ref/mod)
- [Go Vulnerability Database](https://vuln.go.dev/)
- [OWASP Dependency Check](https://owasp.org/www-project-dependency-check/)
- [Software Supply Chain Security](https://www.cisa.gov/supply-chain-risk-management)
- [Secure Software Development Framework (SSDF)](https://csrc.nist.gov/publications/detail/sp/800-218/final)

---

## Appendix: Dependency License Verification

### License Compliance Check

```bash
# Install license checker
go install github.com/google/licensecheck@latest

# Check licenses
licensecheck ./...
```

**Accepted Licenses**:
- MIT
- Apache 2.0
- BSD (2-Clause, 3-Clause)
- ISC
- MPL 2.0

**Check Before Adding New Dependency**:

```bash
# What license does this package use?
go mod download -json github.com/org/module@version | jq .Info.License

# Does it match our whitelist?
# If not, get explicit approval before adding
```
