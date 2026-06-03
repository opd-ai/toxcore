# Security Patch Playbook

**Date**: 2026-06-03  
**Applies To**: toxcore-go project  
**Purpose**: Define procedures and timelines for security patches and vulnerability disclosures

## Table of Contents

1. [Overview](#overview)
2. [Vulnerability Classification](#vulnerability-classification)
3. [Response Timeline](#response-timeline)
4. [Patch Development Process](#patch-development-process)
5. [Release Procedures](#release-procedures)
6. [Communication Plan](#communication-plan)
7. [Rollback Procedures](#rollback-procedures)
8. [Post-Incident Review](#post-incident-review)

---

## Overview

This playbook ensures that security vulnerabilities in toxcore-go are identified, resolved, and communicated to users in a timely manner. The process balances transparency with responsible disclosure.

### Key Principles

1. **User Safety First**: Patches are prioritized by impact and exploitability
2. **Transparency**: Disclose vulnerabilities responsibly to coordinated responsible disclosure
3. **Rapid Response**: Critical vulnerabilities are patched within 48 hours
4. **Clear Communication**: Users are notified before exploitation becomes possible
5. **Community Involvement**: Patches are reviewed by maintainers and community

---

## Vulnerability Classification

### Critical (CVSS 9.0-10.0)

**Definition**: Vulnerabilities allowing remote code execution, complete information disclosure, or total loss of confidentiality/integrity/availability.

**Examples**:
- Memory safety violations (buffer overflow, use-after-free) in core crypto
- Plaintext transmission of sensitive data
- Key leakage in cryptographic operations
- Authentication bypass

**Response SLA**: **24 hours** from report to patch release

**Process**:
1. Immediately notify maintainers (within 1 hour)
2. Begin patch development (within 2 hours)
3. Review patch for correctness (within 6 hours)
4. Release patch (within 24 hours)
5. Publish advisory (within 24 hours)

### High (CVSS 7.0-8.9)

**Definition**: Vulnerabilities allowing significant impact with some conditions (e.g., authenticated attacker, specific configuration).

**Examples**:
- Privilege escalation
- Denial of service with significant resource consumption
- Partial information disclosure
- Weakened cryptographic properties

**Response SLA**: **72 hours** from report to patch release

**Process**:
1. Notify maintainers (within 4 hours)
2. Begin patch development (within 8 hours)
3. Review and test patch (within 24 hours)
4. Release patch (within 72 hours)
5. Publish advisory (within 72 hours)

### Medium (CVSS 4.0-6.9)

**Definition**: Vulnerabilities with moderate impact, typically requiring specific conditions.

**Examples**:
- Information disclosure of non-critical metadata
- Denial of service with normal recovery
- Reduced effectiveness of security features
- Configuration weaknesses

**Response SLA**: **7 days** from report to patch release

**Process**:
1. Notify maintainers (within 24 hours)
2. Add to security backlog (within 2 days)
3. Schedule for next patch cycle (within 7 days)
4. Review and test patch
5. Release in next scheduled update
6. Publish advisory with patch

### Low (CVSS 0.1-3.9)

**Definition**: Vulnerabilities with minimal practical impact.

**Examples**:
- Information disclosure of public data
- Minor denial of service under edge cases
- Usability issues with security implications
- Documentation gaps

**Response SLA**: **30 days** from report to patch release

**Process**:
1. Add to public issue tracker (with security label)
2. Include in next scheduled release cycle
3. May be combined with other fixes
4. Publish advisory when released

---

## Response Timeline

### Day 1 (Critical Only)

- [ ] Vulnerability reported via security contact
- [ ] Verify reproducibility
- [ ] Assess impact and scope
- [ ] Notify maintainers and create private security branch
- [ ] Begin patch development

### Day 2 (Critical Only)

- [ ] Complete patch development
- [ ] Internal code review
- [ ] Compile test suite
- [ ] Run regression tests
- [ ] Prepare draft advisory

### Day 3+ (Critical Released, High In Progress)

- [ ] Release patch as minor/patch version bump
- [ ] Publish security advisory with CVE (if applicable)
- [ ] Notify downstream projects
- [ ] Monitor for exploitation attempts
- [ ] Plan additional hardening measures

### Ongoing (Medium/Low)

- [ ] Process patches in regular release cycle (every 2-4 weeks)
- [ ] Batch similar fixes when possible
- [ ] Publish advisories with release notes
- [ ] Track public disclosure date

---

## Patch Development Process

### 1. Triage & Verification

```
Input: Vulnerability report
├─ Reproduce the issue
├─ Confirm security impact
├─ Assess scope (how many users affected)
├─ Classify severity (Critical/High/Medium/Low)
└─ Create private security issue
```

**Checklist**:
- [ ] Issue is reproducible
- [ ] Root cause identified
- [ ] Impact scope documented
- [ ] Exploitation difficulty assessed

### 2. Branch & Fix

```
git checkout -b security/CVE-XXXX-XXXXX origin/main

# Fix implementation
# Keep commits atomic and descriptive
```

**Guidelines**:
- Use branch naming: `security/CVE-XXXX-XXXXX` or `security/issue-NNN`
- Keep fixes minimal (don't refactor unrelated code)
- Add regression tests for the vulnerability
- Update SECURITY.md with new guidance if needed

### 3. Internal Review

**Reviewers**: At least 2 maintainers + 1 independent reviewer

**Review Checklist**:
- [ ] Fix addresses root cause (not just symptoms)
- [ ] No new vulnerabilities introduced
- [ ] Performance impact acceptable
- [ ] Backward compatibility maintained (or documented)
- [ ] Tests pass (unit + integration)
- [ ] vet/lint checks pass
- [ ] Crypto operations reviewed by crypto expert if applicable

### 4. Testing

```bash
# Full test suite with race detector
go test -race ./...

# Benchmark sensitive paths
go test -bench=. ./crypto ./messaging ./transport

# Fuzz crypto functions
go test -fuzz=FuzzEncrypt ./crypto

# Integration tests with multiple protocol versions
go test -tags integration ./toxnet
```

### 5. Advisory Preparation

**Template**:

```markdown
## Security Advisory: CVE-XXXX-XXXXX

### Title: [Brief Description]

### Affected Versions: X.Y.Z and earlier

### Severity: [CRITICAL|HIGH|MEDIUM|LOW]

### Impact:
[What can an attacker do? What is the user impact?]

### Affected Components:
- Package: messaging
- Functions: SendFriendMessage

### Root Cause:
[Technical explanation]

### Fixed In:
- Version X.Y.Z (Release Date)

### Upgrade Instructions:
1. Update toxcore-go: `go get -u github.com/opd-ai/toxcore@vX.Y.Z`
2. Recompile application
3. Restart service

### Mitigation (Before Upgrading):
[Workarounds if any exist]

### Timeline:
- YYYY-MM-DD: Vulnerability reported
- YYYY-MM-DD: Patch released
- YYYY-MM-DD: Public advisory

### References:
- [CVE Link]
- [Issue Link]
```

---

## Release Procedures

### Pre-Release Checklist

- [ ] All fixes committed to security branch
- [ ] Advisory written and reviewed
- [ ] Version number bumped (major.minor.patch)
- [ ] CHANGELOG.md updated
- [ ] All tests passing
- [ ] Documentation updated (SECURITY.md, etc.)
- [ ] Downstream projects notified (pre-release)

### Release Process

**For Critical (Immediate)**:

```bash
# 1. Create release tag on security branch
git tag -s vX.Y.Z-security -m "Security release"

# 2. Create GitHub release with advisory
# - Highlight as SECURITY RELEASE
# - Provide binary checksums
# - Link to CVE details

# 3. Push to all distribution channels
git push origin vX.Y.Z-security
gh release create vX.Y.Z-security

# 4. Publish to package repositories
go get -u github.com/opd-ai/toxcore@vX.Y.Z-security

# 5. Notify stakeholders
# - Mail to security mailing list
# - Announce on GitHub Discussions
# - Post on community forums
```

**For High/Medium/Low (Regular Cycle)**:

- Include in next scheduled release (every 2-4 weeks)
- Batch multiple fixes when possible
- Include detailed changelog entry

### Post-Release Monitoring

```bash
# Monitor for:
# - Build failures in CI
# - Downstream project issues
# - Exploitation attempts in the wild
# - Reports of patch effectiveness

# Run daily for 1 week:
- Check CI pipeline status
- Review issue tracker for regression reports
- Monitor GitHub Discussions for questions
- Check application logs for exploit patterns
```

---

## Communication Plan

### Notification Recipients

1. **Security Researchers** (responsible disclosure)
   - Contact: security@github.com/opd-ai/toxcore
   - Timing: Pre-release (48-72 hours before public)
   - Method: Email with patch preview

2. **Major Downstream Projects**
   - Projects with 100+ stars depending on toxcore-go
   - List maintained in docs/DOWNSTREAM_PROJECTS.md
   - Timing: 24-48 hours before public release
   - Method: Email + private GitHub issue

3. **Users (via GitHub)**
   - Release notes with SECURITY label
   - GitHub Security advisory feature
   - Timing: With public release

4. **General Public**
   - CVE published if severity warrants
   - Announcement on project channels
   - Social media notification
   - Timing: With public release

### Advisory Distribution

**Release Notes Example**:

```
## v2.1.3 - Security Release (2026-06-10)

### Security Fixes
- **CRITICAL**: Fix plaintext message leakage in async messaging (#1234)
  - Affected: v2.0.0 - v2.1.2
  - Impact: Messages could be sent unencrypted under certain conditions
  - Action: Upgrade immediately
  - CVE: [CVE-XXXX-XXXXX]
  - [Security Advisory](docs/SECURITY_ADVISORIES/CVE-XXXX-XXXXX.md)

- **HIGH**: Fix denial of service in DHT route processing (#1235)
  - Affected: v2.0.0 - v2.1.2
  - Impact: Malicious peers could cause excessive CPU usage
  - Action: Upgrade recommended
```

---

## Rollback Procedures

### When to Rollback

- Patch introduces data loss
- Patch causes widespread crashes
- Patch fails to fix vulnerability
- Patch introduces new critical vulnerability

### Rollback Steps

1. **Assess Impact**
   - How many users updated?
   - Are there known failures?
   - Is there a workaround?

2. **Create Rollback Release**
   ```bash
   # Tag previous stable version
   git tag -s vX.Y.(Z-1) <previous-stable-commit>
   
   # Create release noting rollback reason
   gh release create vX.Y.(Z-1) \
     --prerelease \
     --title "Rollback: vX.Y.Z withdrawn"
   ```

3. **Notify Users**
   - Post urgent notice on GitHub Discussions
   - Send email to mailing list
   - Update release notes with "WITHDRAWN" label
   - Document root cause

4. **Post-Mortem**
   - Why was the issue not caught in testing?
   - What CI improvements are needed?
   - What code review process changes?

---

## Post-Incident Review

### Timeline (1-2 weeks after patch release)

**Attendees**: Core maintainers + assigned reviewers

**Agenda**:

1. **What Went Right**
   - Did communication happen on schedule?
   - Did patch fixes the vulnerability?
   - Were tests adequate?

2. **What Went Wrong**
   - How was vulnerability introduced?
   - Why wasn't it caught earlier?
   - Were there warning signs?

3. **Improvements**
   - Code review process changes
   - Testing gaps to fill
   - Monitoring improvements
   - Documentation clarifications

4. **Action Items**
   - Assign owners
   - Set deadlines
   - Track in project backlog

### Documentation

Create post-incident report and archive:

```
docs/INCIDENT_REPORTS/CVE-XXXX-XXXXX.md
├─ Timeline
├─ Root cause analysis
├─ Affected versions
├─ Action items
└─ Lessons learned
```

---

## Metrics & Tracking

### SLA Compliance

Track for each vulnerability:

| Metric | Target | Threshold |
|--------|--------|-----------|
| Time to first acknowledgment | 24h | < 4h for critical |
| Time to patch | See classification | SLA times above |
| Time to public disclosure | CVSS-dependent | 7 days max for low |
| Response rate | 100% | All reports addressed |

### Reporting

Monthly security report (internal only until disclosure):

```
- # Vulnerabilities reported: X
- # Vulnerabilities patched: Y
- Average time to patch: Z days
- SLA compliance: A%
- Critical issues outstanding: B
```

---

## Resources & Escalation

### Contacts

- **Security Lead**: [designated person]
- **Backup Lead**: [designated person]
- **Release Manager**: [designated person]
- **Crypto Expert**: [designated person]

### Escalation Path

```
Security Report
    ↓
Security Lead (assess & triage)
    ↓
Maintainer Review (if yes)
    ↓
Patch Development (2-3 days)
    ↓
Internal Review & Testing
    ↓
Release Coordinator (notify downstream)
    ↓
Public Release & Disclosure
```

### Tools & Infrastructure

- **Private Issue Tracker**: GitHub Security Advisories
- **Branch Protection**: security/* branches protected, require 2 reviews
- **CI/CD**: Security-specific job that runs enhanced tests
- **Signing**: All security patches signed with project GPG key

---

## Appendix: Vulnerability Reporting

### How Users Report Vulnerabilities

**DO NOT create public GitHub issues for security vulnerabilities**

Instead:

1. **Email**: security@opd-ai (if exists) or GitHub Security Advisory
2. **Include**:
   - Description of vulnerability
   - Affected versions
   - Steps to reproduce (if possible)
   - Your contact information
3. **Expect**: Acknowledgment within 24 hours

### Example Report

```
Subject: Security Report - Plaintext Message Leakage in toxcore-go

Description:
Under specific timing conditions, messages sent to newly-added friends 
may be transmitted unencrypted.

Affected Versions:
- v2.0.0
- v2.0.1
- v2.1.0
- v2.1.1
- v2.1.2

Steps to Reproduce:
1. Create a new Tox instance
2. Add a new friend by public key
3. Immediately send message (before encryption negotiation completes)
4. Observe plaintext in packet capture

Impact:
Users who send sensitive information before encryption negotiation 
completes may have that information transmitted unencrypted.

Severity:
High (requires specific timing, but impacts all users with newly-added friends)
```

---

## See Also

- [SECURITY.md](../SECURITY.md) - Overall security policy
- [SECURE_INTEGRATION_GUIDE.md](./SECURE_INTEGRATION_GUIDE.md) - User security guidance
- [PROFILE_GUIDED_OPTIMIZATION.md](./PROFILE_GUIDED_OPTIMIZATION.md) - Performance guidance
- GitHub Security Advisories: https://github.com/opd-ai/toxcore/security/advisories
