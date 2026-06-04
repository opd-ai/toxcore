// Package toxcore implements the core functionality of the Tox protocol.
// This file exposes the optional Namecoin name resolution layer.
package toxcore

import (
	"github.com/opd-ai/toxcore/nameresolver"
)

// GetNameResolver returns the Resolver configured for this Tox instance.
//
// When Options.NameResolverEnabled is false (the default), the returned resolver
// is a [nameresolver.DisabledResolver] that safely returns
// [nameresolver.ErrNameResolutionDisabled] for every call.
//
// When Options.NameResolverEnabled is true and the name database was opened
// successfully, the returned resolver is a live [nameresolver.NmcdResolver]
// backed by a local bbolt NameDatabase.  Callers can type-assert to
// *nameresolver.NmcdResolver to access extended methods such as
// SetCurrentBlockHeight and LookupBootstrapNodes.
//
// The resolver's lifetime is tied to the Tox instance; it is closed
// automatically when Kill() is called.
func (t *Tox) GetNameResolver() nameresolver.Resolver {
	return t.nameResolver
}
