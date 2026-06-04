package nameresolver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/opd-ai/nmcd/namedb"
	"github.com/opd-ai/toxcore/nameresolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeToxID returns a syntactically valid 76-character hex Tox ID string.
func fakeToxID(seed byte) string {
	const chars = "0123456789abcdef"
	b := make([]byte, 76)
	for i := range b {
		b[i] = chars[(int(seed)+i)%16]
	}
	return string(b)
}

func marshalValue(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestDisabledResolver_AllMethodsReturnDisabled(t *testing.T) {
	r := nameresolver.DisabledResolver{}
	ctx := context.Background()

	_, err := r.LookupToxID(ctx, "alice")
	assert.ErrorIs(t, err, nameresolver.ErrNameResolutionDisabled)

	err = r.RegisterName(ctx, "alice", fakeToxID(0), nil)
	assert.ErrorIs(t, err, nameresolver.ErrNameResolutionDisabled)

	err = r.RenewName(ctx, "alice", nil)
	assert.ErrorIs(t, err, nameresolver.ErrNameResolutionDisabled)

	assert.NoError(t, r.Close())
}

func TestValidateName(t *testing.T) {
	valid := []string{"alice", "bob-jones", "node_1", "a", "x23"}
	for _, n := range valid {
		assert.True(t, nameresolver.ValidateName(n), "expected valid: %q", n)
	}

	invalid := []string{"", "Alice", "a b", "tox/d/x", "../etc", "a!b",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} // 64 chars
	for _, n := range invalid {
		assert.False(t, nameresolver.ValidateName(n), "expected invalid: %q", n)
	}
}

func TestParseToxNameValue(t *testing.T) {
	id := fakeToxID(1)
	raw := marshalValue(map[string]interface{}{"toxid": id})
	v, err := nameresolver.ParseToxNameValue(raw)
	require.NoError(t, err)
	assert.Equal(t, id, v.ToxID)

	// short toxid
	_, err = nameresolver.ParseToxNameValue(`{"toxid":"short"}`)
	assert.ErrorIs(t, err, nameresolver.ErrInvalidToxID)

	// malformed JSON
	_, err = nameresolver.ParseToxNameValue(`{bad}`)
	assert.Error(t, err)
}

func TestParseBootstrapNameValue(t *testing.T) {
	id := fakeToxID(2)
	raw := marshalValue(map[string]interface{}{"toxid": id, "addr": "node.example.com:33445", "net": "udp4"})
	v, err := nameresolver.ParseBootstrapNameValue(raw)
	require.NoError(t, err)
	assert.Equal(t, id, v.ToxID)
	assert.Equal(t, "node.example.com:33445", v.Addr)
	assert.Equal(t, "udp4", v.Net)
}

// openTestDB creates a temp NameDatabase pre-populated with the given records
// and returns an NmcdResolver backed by it.
func openTestResolver(t *testing.T, records map[string]namedb.NameRecord) *nameresolver.NmcdResolver {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "names.db")

	db, err := namedb.NewNameDatabase(dbPath)
	require.NoError(t, err)

	for name, rec := range records {
		r := rec
		r.Name = name
		require.NoError(t, db.PutName(name, &r))
	}
	require.NoError(t, db.Close())

	resolver, err := nameresolver.NewNmcdResolver(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resolver.Close() })
	return resolver
}

func zeroHash() chainhash.Hash { return chainhash.Hash{} }

func TestNmcdResolver_LookupToxID_Found(t *testing.T) {
	id := fakeToxID(3)
	rec := namedb.NameRecord{
		Value:     marshalValue(map[string]interface{}{"toxid": id}),
		Height:    100,
		ExpiresAt: 36100,
		TxHash:    zeroHash(),
	}
	r := openTestResolver(t, map[string]namedb.NameRecord{"tox/d/alice": rec})

	got, err := r.LookupToxID(context.Background(), "alice")
	require.NoError(t, err)
	assert.Equal(t, id, got)
}

func TestNmcdResolver_LookupToxID_NotFound(t *testing.T) {
	r := openTestResolver(t, nil)
	_, err := r.LookupToxID(context.Background(), "nobody")
	assert.ErrorIs(t, err, nameresolver.ErrNameNotFound)
}

func TestNmcdResolver_LookupToxID_InvalidName(t *testing.T) {
	r := openTestResolver(t, nil)
	_, err := r.LookupToxID(context.Background(), "UPPER")
	assert.ErrorIs(t, err, nameresolver.ErrInvalidName)
}

func TestNmcdResolver_LookupToxID_Expired(t *testing.T) {
	id := fakeToxID(4)
	rec := namedb.NameRecord{
		Value:     marshalValue(map[string]interface{}{"toxid": id}),
		Height:    100,
		ExpiresAt: 200, // expires at block 200
		TxHash:    zeroHash(),
	}
	r := openTestResolver(t, map[string]namedb.NameRecord{"tox/d/old": rec})
	r.SetCurrentBlockHeight(300) // chain is past expiry

	_, err := r.LookupToxID(context.Background(), "old")
	assert.ErrorIs(t, err, nameresolver.ErrNameExpired)
}

func TestNmcdResolver_LookupToxID_ExpirySkippedWhenHeightUnknown(t *testing.T) {
	id := fakeToxID(5)
	rec := namedb.NameRecord{
		Value:     marshalValue(map[string]interface{}{"toxid": id}),
		Height:    100,
		ExpiresAt: 200,
		TxHash:    zeroHash(),
	}
	r := openTestResolver(t, map[string]namedb.NameRecord{"tox/d/unknownheight": rec})
	// currentBlock defaults to 0 → expiry checking disabled

	got, err := r.LookupToxID(context.Background(), "unknownheight")
	require.NoError(t, err)
	assert.Equal(t, id, got)
}

func TestNmcdResolver_LookupBootstrapNodes(t *testing.T) {
	id := fakeToxID(6)
	rec := namedb.NameRecord{
		Value: marshalValue(map[string]interface{}{
			"toxid": id,
			"addr":  "bootstrap.example.com:33445",
			"net":   "udp4",
		}),
		Height:    100,
		ExpiresAt: 36100,
		TxHash:    zeroHash(),
	}
	r := openTestResolver(t, map[string]namedb.NameRecord{"tox/bootstrap/main": rec})

	nodes, err := r.LookupBootstrapNodes(context.Background())
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, id, nodes[0].ToxID)
	assert.Equal(t, "bootstrap.example.com:33445", nodes[0].Addr)
}

func TestNmcdResolver_RegisterAndRenew_NotSupported(t *testing.T) {
	r := openTestResolver(t, nil)
	ctx := context.Background()

	err := r.RegisterName(ctx, "alice", fakeToxID(7), nil)
	assert.ErrorIs(t, err, nameresolver.ErrRegistrationNotSupported)

	err = r.RenewName(ctx, "alice", nil)
	assert.ErrorIs(t, err, nameresolver.ErrRegistrationNotSupported)
}

func TestNmcdResolver_ContextCancelled(t *testing.T) {
	r := openTestResolver(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.LookupToxID(ctx, "alice")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNmcdResolver_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "names.db")

	// Pre-create an empty database so NewNmcdResolver succeeds.
	db, err := namedb.NewNameDatabase(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	r, err := nameresolver.NewNmcdResolver(dbPath)
	require.NoError(t, err)

	assert.NoError(t, r.Close())
	// Second close should not panic or return an unexpected error.
	assert.NoError(t, r.Close())
}

// TestNewNmcdResolver_MissingDir verifies that NewNmcdResolver returns an error
// when the parent directory does not exist.
func TestNewNmcdResolver_MissingDir(t *testing.T) {
	_, err := nameresolver.NewNmcdResolver(filepath.Join(os.TempDir(), "nonexistent-tox-dir-xyz", "names.db"))
	assert.Error(t, err)
}
