// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"strings"
	"testing"

	"gitea.dev/models/unittest"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSynchronizeGPGKeys(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	const ownerID, sourceID, otherSourceID int64 = 2, 41, 42

	newPublicKey := func(email string) (keyID, content string) {
		t.Helper()
		entity, err := openpgp.NewEntity("LDAP test", "", email, &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
		require.NoError(t, err)
		var armored strings.Builder
		writer, err := armor.Encode(&armored, openpgp.PublicKeyType, nil)
		require.NoError(t, err)
		require.NoError(t, entity.Serialize(writer))
		require.NoError(t, writer.Close())
		return entity.PrimaryKey.KeyIdString(), armored.String()
	}

	manualID, manual := newPublicKey("user2@example.com")
	managedID, managed := newPublicKey("user2@example.com")
	otherID, other := newPublicKey("user2@example.com")
	wrongEmailID, wrongEmail := newPublicKey("user1@example.com")
	_, err := AddGPGKey(ctx, ownerID, manual, "", "")
	require.NoError(t, err)
	require.NoError(t, SynchronizeGPGKeys(ctx, ownerID, otherSourceID, []string{other}))

	values := []string{managed, manual, managed}
	require.NoError(t, SynchronizeGPGKeys(ctx, ownerID, sourceID, values))
	keys, err := FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NoError(t, keys[0].LoadSubKeys(ctx))
	require.Len(t, keys[0].SubsKey, 1)
	subkeyID := keys[0].SubsKey[0].KeyID
	assert.Equal(t, sourceID, keys[0].LoginSourceID)
	assert.False(t, keys[0].Verified)
	imported, err := GPGKeyToEntity(ctx, keys[0])
	require.NoError(t, err)
	assert.Equal(t, managedID, imported.PrimaryKey.KeyIdString())

	require.ErrorContains(t, SynchronizeGPGKeys(ctx, ownerID, sourceID, append(values, wrongEmail, "invalid armor")), "none of the emails")
	rejected, err := FindGPGKeyWithSubKeys(ctx, wrongEmailID)
	require.NoError(t, err)
	assert.Empty(t, rejected)
	repeated, err := FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	require.Len(t, repeated, 1)
	assert.Equal(t, keys[0].ID, repeated[0].ID)

	require.NoError(t, SynchronizeGPGKeys(ctx, ownerID, sourceID, nil))
	keys, err = FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	assert.Empty(t, keys)
	keys, err = FindGPGKeyWithSubKeys(ctx, subkeyID)
	require.NoError(t, err)
	assert.Empty(t, keys)
	_, err = GetGPGImportByKeyID(ctx, managedID)
	assert.True(t, IsErrGPGKeyImportNotExist(err))
	for _, keyID := range []string{manualID, otherID} {
		keys, err := FindGPGKeyWithSubKeys(ctx, keyID)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	}
}
