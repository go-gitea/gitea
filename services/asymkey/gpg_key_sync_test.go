// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey_test

import (
	"strings"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/asymkey"
	"gitea.dev/services/auth/source/ldap"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSynchronizeGPGKeys(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	const ownerID, otherSourceID int64 = 2, 42
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: ownerID})
	source := &auth_model.Source{Type: auth_model.LDAP, Name: "test LDAP", Cfg: &ldap.Source{AttributeGPGPublicKey: "gpgPublicKey"}}
	require.NoError(t, auth_model.CreateSource(ctx, source))
	sourceID := source.ID
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	config := &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA}
	newEntity := func(email string) *openpgp.Entity {
		t.Helper()
		entity, err := openpgp.NewEntity("LDAP test", "", email, config)
		require.NoError(t, err)
		return entity
	}
	armorKey := func(entity *openpgp.Entity) string {
		t.Helper()
		var armored strings.Builder
		writer, err := armor.Encode(&armored, openpgp.PublicKeyType, nil)
		require.NoError(t, err)
		require.NoError(t, entity.Serialize(writer))
		require.NoError(t, writer.Close())
		return armored.String()
	}

	manualEntity := newEntity("user2@example.com")
	manualID, manual := manualEntity.PrimaryKey.KeyIdString(), armorKey(manualEntity)
	managedEntity := newEntity("user2@example.com")
	managedID, managed := managedEntity.PrimaryKey.KeyIdString(), armorKey(managedEntity)
	otherEntity := newEntity("user2@example.com")
	otherID, other := otherEntity.PrimaryKey.KeyIdString(), armorKey(otherEntity)
	wrongEmailEntity := newEntity("user1@example.com")
	wrongEmailID, wrongEmail := wrongEmailEntity.PrimaryKey.KeyIdString(), armorKey(wrongEmailEntity)
	_, err := asymkey_model.AddGPGKey(ctx, ownerID, manual, "", "")
	require.NoError(t, err)
	require.NoError(t, asymkey.SynchronizeGPGKeys(ctx, user, otherSourceID, []string{other}))

	values := []string{managed, manual, managed}
	require.NoError(t, asymkey.SynchronizeGPGKeys(ctx, user, sourceID, values))
	keys, err := asymkey_model.FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NoError(t, keys[0].LoadSubKeys(ctx))
	require.Len(t, keys[0].SubsKey, 1)
	subkeyID := keys[0].SubsKey[0].KeyID
	assert.Equal(t, sourceID, keys[0].LoginSourceID)
	assert.False(t, keys[0].Verified)
	imported, err := asymkey_model.GPGKeyToEntity(ctx, keys[0])
	require.NoError(t, err)
	assert.Equal(t, managedID, imported.PrimaryKey.KeyIdString())

	require.ErrorContains(t, asymkey.SynchronizeGPGKeys(ctx, user, sourceID, append(values, wrongEmail, "invalid armor")), "none of the emails")
	rejected, err := asymkey_model.FindGPGKeyWithSubKeys(ctx, wrongEmailID)
	require.NoError(t, err)
	assert.Empty(t, rejected)
	repeated, err := asymkey_model.FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	require.Len(t, repeated, 1)
	assert.Equal(t, keys[0].ID, repeated[0].ID)

	for _, id := range []int64{keys[0].ID, keys[0].SubsKey[0].ID} {
		require.ErrorIs(t, asymkey.DeleteGPGKey(ctx, user, id), asymkey.ErrGPGKeyExternallyManaged)
	}
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.ErrorContains(t, asymkey.SynchronizeGPGKeys(ctx, otherUser, sourceID, []string{managed}), managedID)

	_, err = db.GetEngine(ctx).ID(keys[0].ID).Cols("verified").Update(&asymkey_model.GPGKey{Verified: true})
	require.NoError(t, err)
	managedEntity.Subkeys = nil
	require.NoError(t, managedEntity.AddSigningSubkey(config))
	require.NoError(t, managedEntity.AddUserId("Second UID", "", "user2@example.com", config))
	updated := armorKey(managedEntity)
	require.NoError(t, asymkey.SynchronizeGPGKeys(ctx, user, sourceID, []string{updated}))
	keys, err = asymkey_model.FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.True(t, keys[0].Verified)
	require.NoError(t, keys[0].LoadSubKeys(ctx))
	require.Len(t, keys[0].SubsKey, 1)
	assert.Equal(t, managedEntity.Subkeys[0].PublicKey.KeyIdString(), keys[0].SubsKey[0].KeyID)
	oldSubs, err := asymkey_model.FindGPGKeyWithSubKeys(ctx, subkeyID)
	require.NoError(t, err)
	assert.Empty(t, oldSubs)
	exported, err := asymkey_model.GPGKeyToEntity(ctx, keys[0])
	require.NoError(t, err)
	require.Len(t, exported.Identities, 2)
	require.Len(t, exported.Subkeys, 1)
	assert.Equal(t, managedEntity.Subkeys[0].PublicKey.KeyIdString(), exported.Subkeys[0].PublicKey.KeyIdString())
	require.NoError(t, asymkey.SynchronizeGPGKeys(ctx, user, sourceID, []string{updated}))

	require.NoError(t, asymkey.SynchronizeGPGKeys(ctx, user, sourceID, nil))
	var events []audit_model.Event
	require.NoError(t, db.GetEngine(ctx).Where("actor_id=? AND scope_id=?", user_model.NewAuthSourceUser().ID, ownerID).Asc("id").Find(&events))
	var actions []audit_model.Action
	for _, event := range events {
		if audit_model.DecodeMetadata(event.Metadata)["gpg_key_id"] == managedID {
			actions = append(actions, event.Action)
		}
	}
	assert.Equal(t, []audit_model.Action{audit_model.UserKeyGPGAdd, audit_model.UserKeyGPGRemove, audit_model.UserKeyGPGAdd, audit_model.UserKeyGPGRemove}, actions)
	keys, err = asymkey_model.FindGPGKeyWithSubKeys(ctx, managedID)
	require.NoError(t, err)
	assert.Empty(t, keys)
	keys, err = asymkey_model.FindGPGKeyWithSubKeys(ctx, subkeyID)
	require.NoError(t, err)
	assert.Empty(t, keys)
	_, err = asymkey_model.GetGPGImportByKeyID(ctx, managedID)
	assert.True(t, asymkey_model.IsErrGPGKeyImportNotExist(err))
	for _, keyID := range []string{manualID, otherID} {
		keys, err := asymkey_model.FindGPGKeyWithSubKeys(ctx, keyID)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	}
}
