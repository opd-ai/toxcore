package nameresolver

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"regexp"
)

// Sentinel errors returned by Resolver implementations.
var (
	// ErrNameResolutionDisabled is returned by DisabledResolver for every call.
	// It indicates that name resolution has not been enabled in Options.
	ErrNameResolutionDisabled = errors.New("name resolution is disabled")

	// ErrNameNotFound is returned when the requested name has no record in the
	// local name database.
	ErrNameNotFound = errors.New("name not found")

	// ErrNameExpired is returned when the name record exists but its on-chain
	// registration has expired (ExpiresAt block height has passed).
	ErrNameExpired = errors.New("name record has expired")

	// ErrInvalidName is returned when the supplied name does not conform to the
	// allowed character set [a-z0-9_-]{1,63}.
	ErrInvalidName = errors.New("invalid name: must match [a-z0-9_-]{1,63}")

	// ErrInvalidToxID is returned when the ToxID embedded in a name record is
	// not a valid 76-character hex string.
	ErrInvalidToxID = errors.New("invalid ToxID in name record")

	// ErrRegistrationNotSupported is returned by Phase 1 implementations that
	// do not yet support on-chain name registration.
	ErrRegistrationNotSupported = errors.New("on-chain name registration not yet supported in this build")
)

// validNameRE matches names that may appear in the tox/d/ and tox/bootstrap/
// Namecoin namespaces.  Only lower-case alphanumerics, hyphens, and underscores
// are permitted; length is 1–63 characters.
var validNameRE = regexp.MustCompile(`^[a-z0-9_-]{1,63}$`)

// ValidateName reports whether name is a legal Tox Namecoin name label.
func ValidateName(name string) bool {
	return validNameRE.MatchString(name)
}

// ToxNameValue is the JSON structure stored as the Namecoin record value for
// tox/d/<username> entries.
type ToxNameValue struct {
	// ToxID is the 76-character hex Tox address bound to this name.
	ToxID string `json:"toxid"`

	// Transports is an optional list of transport hints (e.g. ".onion",
	// ".b32.i2p", "host:port").  May be nil or empty.
	Transports []string `json:"transports,omitempty"`
}

// ParseToxNameValue decodes a raw JSON name-record value into a ToxNameValue.
func ParseToxNameValue(raw string) (*ToxNameValue, error) {
	var v ToxNameValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	if len(v.ToxID) != 76 {
		return nil, ErrInvalidToxID
	}
	return &v, nil
}

// BootstrapNameValue is the JSON structure stored as the Namecoin record value
// for tox/bootstrap/<tag> entries.
type BootstrapNameValue struct {
	// ToxID is the 76-character hex Tox address of the bootstrap node.
	ToxID string `json:"toxid"`

	// Addr is the network address of the bootstrap node in host:port form.
	Addr string `json:"addr"`

	// Net is the network type: "udp4", "onion", "i2p", etc.
	Net string `json:"net"`
}

// ParseBootstrapNameValue decodes a raw JSON name-record value into a
// BootstrapNameValue.
func ParseBootstrapNameValue(raw string) (*BootstrapNameValue, error) {
	var v BootstrapNameValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	if len(v.ToxID) != 76 {
		return nil, ErrInvalidToxID
	}
	return &v, nil
}

// Resolver is the interface consumed by toxcore's friend management layer to
// resolve human-readable Namecoin names to Tox IDs.
//
// All methods accept a context so that callers can apply timeouts or
// cancellation.  Implementations must honour context cancellation.
type Resolver interface {
	// LookupToxID resolves a Namecoin name (without the "tox/d/" prefix) to the
	// 76-character hex Tox ID bound to that name.
	//
	// Returns ErrNameResolutionDisabled when name resolution is disabled,
	// ErrNameNotFound when no record exists, ErrNameExpired when the record has
	// expired, and ErrInvalidName when name fails validation.
	LookupToxID(ctx context.Context, name string) (string, error)

	// RegisterName publishes a new tox/d/<name> → toxID binding to the
	// Namecoin blockchain.  key is the ECDSA private key used to sign and fund
	// the NAME_NEW transaction.
	//
	// Phase 1 implementations return ErrRegistrationNotSupported.
	RegisterName(ctx context.Context, name, toxID string, key *ecdsa.PrivateKey) error

	// RenewName re-publishes the tox/d/<name> binding before expiry.
	// key is the ECDSA private key that owns the current registration.
	//
	// Phase 1 implementations return ErrRegistrationNotSupported.
	RenewName(ctx context.Context, name string, key *ecdsa.PrivateKey) error

	// Close releases any resources held by the resolver (e.g. database
	// handles).  It is safe to call Close multiple times.
	Close() error
}
