package async

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/require"
)

func TestCheckAndRotateKeysPreservesPreviousIdentityMaterial(t *testing.T) {
	t.Parallel()

	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	original := *keyPair
	client, err := NewClientWithKeyRotation(keyPair, nil, 24*time.Hour)
	require.NoError(t, err)
	defer close(client.stopChan)

	client.keyRotation.KeyCreationTime = time.Now().Add(-31 * 24 * time.Hour)

	client.checkAndRotateKeys()

	identities := client.GetAllActiveIdentities()
	require.Len(t, identities, 2)
	require.Equal(t, original.Public, identities[1].Public)
	require.Equal(t, original.Private, identities[1].Private)
}
