# Security Gaps — 2026-06-03

This document records gaps between `toxcore-go`'s **stated security goals** (`SECURITY.md`,
`README.md`) and the security controls present in the current tree. Each gap is verified
against specific files and line numbers and ordered by risk. It complements `AUDIT.md`, which
records concrete findings with exploit data flows.

**Re-verification note:** A prior `GAPS.md` documented gaps including the `toxnet`
`RequireEncryption` plaintext bypass, UPnP LOCATION SSRF, ratchet decrypt-then-commit, and
forward-secrecy accounting. The security-relevant ones were re-checked and found
**remediated** in the current code — see `AUDIT.md` → "Previously Reported — Re-verified as
Fixed". The gaps below are those that remain open from a *security-posture* standpoint.

## Gap 1 — Untrusted relays are a stated threat, but one decode path lacks input bounds

- **Stated Goal:** `SECURITY.md` lists "Denial-of-service attacks on core protocol state" as
  in-scope, and the async design (`README.md`, `async/doc.go`) treats **storage/relay nodes as
  untrusted** — forward secrecy, one-time pre-keys, and epoch pseudonyms exist because relays
  may be malicious.
- **Current State:** Most network decoders are bounded (relay 64 KiB, TCP 1 MiB, mux frames,
  async store-side `MaxMessageSize` checks at `async/storage.go:278,609`). However the
  retrieve-response handler decodes attacker-controlled bytes from a storage node with
  `encoding/gob` and **no length or element-count cap**
  (`async/client.go:1267 → 1313 → 1320`). `gob` is documented as unsafe against adversarial
  input. (Tracked as `AUDIT.md` finding **M-1**.)
- **Risk:** A malicious or compromised storage node that a client retrieves from can send a
  crafted `gob` payload that induces disproportionate allocation, causing a memory-pressure
  DoS on the client. Bounded to 2 KiB on UDP (`transport/udp.go:208`) but reachable with up to
  1 MiB over TCP-relay transport.
- **Closing the Gap:** Validate `packet.Data` with `limits.ValidateProcessingBuffer` (or a
  tighter async bound) before decoding, cap the decoded `[]*ObfuscatedAsyncMessage` length, and
  prefer the project's existing length-prefixed binary framing over `gob` for network-facing
  decode. Add a regression test that rejects an oversized-count `gob` payload.

## Gap 2 — UPnP SSRF hardening (M-06) stops at the gateway URL, not the derived control URL

- **Stated Goal:** The M-06 remediation (`transport/upnp_client.go:126-180`) establishes the
  project convention that URLs used by the NAT-traversal client must be validated against an
  http/https scheme and a private/LAN-IP allowlist before requests are issued.
- **Current State:** The SSDP `LOCATION`/gateway URL is validated, but the `<controlURL>`
  extracted from the gateway's device-description XML is resolved via `baseURL.Parse` and used
  for SOAP `POST`s **without** re-applying the private-IP check
  (`transport/upnp_client.go:295-305,399`). (Tracked as `AUDIT.md` finding **L-1**.)
- **Risk:** A LAN-positioned attacker (or a spoofed SSDP response) can point the client's SOAP
  requests at an arbitrary host — a LAN-adjacent SSRF / request-redirection primitive.
  Redirects are already disabled, which limits but does not close the gap.
- **Closing the Gap:** Re-run `validateUPnPLocationURL` (scheme + `isPrivateIP`) on the final
  resolved `controlURL` and reject non-private/non-loopback hosts. Add a unit test with an XML
  body whose `<controlURL>` resolves to a public IP and assert rejection.

## Gap 3 — No automated dependency-vulnerability gate

- **Stated Goal:** `SECURITY.md` commits to a CVE-assignment process and a 90-day fix SLA for
  Critical/High issues, implying active tracking of vulnerabilities — including those reaching
  the project through its dependencies.
- **Current State:** There is no `govulncheck` step in CI, and it could not be run in this
  environment because outbound DNS to `vuln.go.dev` is blocked. Dependency CVEs are therefore
  not detected automatically. (Tracked as `AUDIT.md` finding **L-2**.)
- **Risk:** A future advisory in `golang.org/x/crypto`, `golang.org/x/net`, `flynn/noise`, or a
  transitive module would go unnoticed until manually checked, widening the exposure window
  against the project's own SLA commitments.
- **Closing the Gap:** Add a `govulncheck ./...` job to the GitHub Actions workflow (in an
  environment with egress to `vuln.go.dev`) and fail the build on known advisories. Optionally
  enable Dependabot/`go mod` update automation.

## Gap 4 — "Experimental, un-audited" status is documented but easy to miss for embedders

- **Stated Goal:** `SECURITY.md` → "External Audit Status" states no third-party audit has been
  performed and the library should be treated as **experimental** and unsuitable for
  high-stakes production use until audited.
- **Current State:** This caveat lives only in `SECURITY.md`; the `README.md` feature list and
  GoDoc present the cryptography as production-ready without surfacing the un-audited status at
  the point a developer first integrates the library.
- **Risk:** Integrators may deploy the library in a threat model it is not yet validated for,
  assuming the rich crypto feature set implies external assurance.
- **Closing the Gap:** Surface the "experimental / pending third-party audit" notice in the
  `README.md` security/usage section and in the package GoDoc (`doc.go`), linking to
  `SECURITY.md`. (Documentation change only.)
