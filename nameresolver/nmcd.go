package nameresolver

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/opd-ai/nmcd/namedb"
)

const (
	// toxNamePrefix is the Namecoin key prefix for Tox identity records.
	toxNamePrefix = "tox/d/"

	// toxBootstrapPrefix is the Namecoin key prefix for bootstrap node records.
	toxBootstrapPrefix = "tox/bootstrap/"
)

// NmcdResolver implements [Resolver] using a local nmcd [namedb.NameDatabase].
//
// The database is a bbolt file populated externally by the nmcd Namecoin node
// (SPV or full).  NmcdResolver is read-only with respect to the chain: it does
// not initiate any Namecoin transactions.  Registration and renewal operations
// (RegisterName, RenewName) return [ErrRegistrationNotSupported] in Phase 1.
//
// NmcdResolver is safe for concurrent use.
type NmcdResolver struct {
	db           *namedb.NameDatabase
	mu           sync.RWMutex
	currentBlock int32 // last known chain tip height; 0 means unknown
}

// Compile-time assertion that *NmcdResolver satisfies Resolver.
var _ Resolver = (*NmcdResolver)(nil)

// NewNmcdResolver opens (or creates) a NameDatabase at dbPath and returns a
// ready-to-use NmcdResolver.
//
// dbPath should be a file path such as "/var/lib/tox/names.db".  The directory
// must already exist.  Multiple processes must not open the same file
// simultaneously (bbolt enforces exclusive locking).
func NewNmcdResolver(dbPath string) (*NmcdResolver, error) {
	db, err := namedb.NewNameDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("nameresolver: open database %q: %w", dbPath, err)
	}
	return &NmcdResolver{db: db}, nil
}

// SetCurrentBlockHeight updates the resolver's view of the current chain tip.
// The height is used to evaluate whether a name record has expired.
// Pass 0 to disable expiry checking (expiry will be skipped with a warning).
func (r *NmcdResolver) SetCurrentBlockHeight(height int32) {
	r.mu.Lock()
	r.currentBlock = height
	r.mu.Unlock()
}

// LookupToxID resolves name (without the "tox/d/" prefix) to a 76-character
// hex Tox ID.
//
// The lookup sequence is:
//  1. Validate name against [ValidateName].
//  2. Query the local NameDatabase for the key "tox/d/<name>".
//  3. If a record is found and the current block height is known, reject
//     expired records with [ErrNameExpired].
//  4. Parse the record's JSON value and return the embedded ToxID.
func (r *NmcdResolver) LookupToxID(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if !ValidateName(name) {
		return "", ErrInvalidName
	}

	key := toxNamePrefix + name

	r.mu.RLock()
	currentBlock := r.currentBlock
	r.mu.RUnlock()

	rec, err := r.db.GetName(key)
	if err != nil {
		if errors.Is(err, namedb.ErrNameNotFound) {
			return "", ErrNameNotFound
		}
		return "", fmt.Errorf("nameresolver: database error: %w", err)
	}
	if rec == nil {
		return "", ErrNameNotFound
	}

	if currentBlock > 0 && rec.ExpiresAt > 0 && currentBlock >= rec.ExpiresAt {
		return "", ErrNameExpired
	}

	v, err := ParseToxNameValue(rec.Value)
	if err != nil {
		return "", fmt.Errorf("nameresolver: malformed record value for %q: %w", name, err)
	}

	return v.ToxID, nil
}

// LookupBootstrapNodes returns all tox/bootstrap/* records from the local
// NameDatabase.  Records that fail to parse or are expired (given currentBlock)
// are silently skipped.
//
// Returns an empty slice when the database contains no valid bootstrap records.
func (r *NmcdResolver) LookupBootstrapNodes(ctx context.Context) ([]BootstrapNameValue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	currentBlock := r.currentBlock
	r.mu.RUnlock()

	records, err := r.db.ScanNames(toxBootstrapPrefix, 1000)
	if err != nil {
		return nil, fmt.Errorf("nameresolver: scan bootstrap records: %w", err)
	}

	var out []BootstrapNameValue
	for _, rec := range records {
		if !strings.HasPrefix(rec.Name, toxBootstrapPrefix) {
			continue
		}
		if currentBlock > 0 && rec.ExpiresAt > 0 && currentBlock >= rec.ExpiresAt {
			continue
		}
		v, err := ParseBootstrapNameValue(rec.Value)
		if err != nil {
			continue
		}
		out = append(out, *v)
	}
	return out, nil
}

// RegisterName returns ErrRegistrationNotSupported.
// On-chain name registration is not implemented in Phase 1.
func (r *NmcdResolver) RegisterName(_ context.Context, _, _ string, _ *ecdsa.PrivateKey) error {
	return ErrRegistrationNotSupported
}

// RenewName returns ErrRegistrationNotSupported.
// On-chain name renewal is not implemented in Phase 1.
func (r *NmcdResolver) RenewName(_ context.Context, _ string, _ *ecdsa.PrivateKey) error {
	return ErrRegistrationNotSupported
}

// Close releases the underlying database handle.
func (r *NmcdResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.db.Close()
}
