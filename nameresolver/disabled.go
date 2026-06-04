package nameresolver

import (
	"context"
	"crypto/ecdsa"
)

// DisabledResolver is a safe, zero-allocation stub that implements [Resolver]
// and returns [ErrNameResolutionDisabled] for every call.
//
// It is the default resolver used by toxcore when name resolution has not been
// opted in via Options.NameResolverEnabled.  Applications that do not require
// Namecoin name resolution incur zero overhead from this package.
type DisabledResolver struct{}

// Compile-time assertion that DisabledResolver satisfies Resolver.
var _ Resolver = DisabledResolver{}

// LookupToxID always returns ErrNameResolutionDisabled.
func (DisabledResolver) LookupToxID(_ context.Context, _ string) (string, error) {
	return "", ErrNameResolutionDisabled
}

// RegisterName always returns ErrNameResolutionDisabled.
func (DisabledResolver) RegisterName(_ context.Context, _, _ string, _ *ecdsa.PrivateKey) error {
	return ErrNameResolutionDisabled
}

// RenewName always returns ErrNameResolutionDisabled.
func (DisabledResolver) RenewName(_ context.Context, _ string, _ *ecdsa.PrivateKey) error {
	return ErrNameResolutionDisabled
}

// Close is a no-op for the disabled resolver.
func (DisabledResolver) Close() error { return nil }
