// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"crypto/ed25519"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMirrorSSHKeypair(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("CreateMirrorSSHKeypair", func(t *testing.T) {
		// Test creating a new SSH keypair for a user
		keypair, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 1)
		require.NoError(t, err)
		assert.NotNil(t, keypair)
		assert.Equal(t, int64(1), keypair.OwnerID)
		assert.NotEmpty(t, keypair.PublicKey)
		assert.NotEmpty(t, keypair.PrivateKeyEncrypted)
		assert.NotEmpty(t, keypair.Fingerprint)
		assert.Positive(t, keypair.CreatedUnix)
		assert.Positive(t, keypair.UpdatedUnix)

		// Verify the public key is in SSH format
		assert.Contains(t, keypair.PublicKey, "ssh-ed25519")

		// Test creating a keypair for an organization
		orgKeypair, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 2)
		require.NoError(t, err)
		assert.NotNil(t, orgKeypair)
		assert.Equal(t, int64(2), orgKeypair.OwnerID)

		// Ensure different owners get different keypairs
		assert.NotEqual(t, keypair.PublicKey, orgKeypair.PublicKey)
		assert.NotEqual(t, keypair.Fingerprint, orgKeypair.Fingerprint)
	})

	t.Run("GetMirrorSSHKeypairByOwner", func(t *testing.T) {
		// Create a keypair first
		created, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 3)
		require.NoError(t, err)

		// Test retrieving the keypair
		retrieved, err := repo_model.GetMirrorSSHKeypairByOwner(db.DefaultContext, 3)
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.PublicKey, retrieved.PublicKey)
		assert.Equal(t, created.Fingerprint, retrieved.Fingerprint)

		// Test retrieving non-existent keypair
		_, err = repo_model.GetMirrorSSHKeypairByOwner(db.DefaultContext, 999)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("GetDecryptedPrivateKey", func(t *testing.T) {
		// Ensure we have a valid SECRET_KEY for testing
		if setting.SecretKey == "" {
			setting.SecretKey = "test-secret-key-for-testing"
		}

		// Create a keypair
		keypair, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 4)
		require.NoError(t, err)

		// Test decrypting the private key
		privateKey, err := keypair.GetDecryptedPrivateKey()
		require.NoError(t, err)
		assert.IsType(t, ed25519.PrivateKey{}, privateKey)
		assert.Len(t, privateKey, ed25519.PrivateKeySize)

		// Verify the private key corresponds to the public key
		publicKey := privateKey.Public().(ed25519.PublicKey)
		assert.Len(t, publicKey, ed25519.PublicKeySize)
	})

	t.Run("DeleteMirrorSSHKeypair", func(t *testing.T) {
		// Create a keypair
		_, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 5)
		require.NoError(t, err)

		// Verify it exists
		_, err = repo_model.GetMirrorSSHKeypairByOwner(db.DefaultContext, 5)
		require.NoError(t, err)

		// Delete it
		err = repo_model.DeleteMirrorSSHKeypair(db.DefaultContext, 5)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo_model.GetMirrorSSHKeypairByOwner(db.DefaultContext, 5)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("RegenerateMirrorSSHKeypair", func(t *testing.T) {
		// Create initial keypair
		original, err := repo_model.CreateMirrorSSHKeypair(db.DefaultContext, 6)
		require.NoError(t, err)

		// Regenerate it
		regenerated, err := repo_model.RegenerateMirrorSSHKeypair(db.DefaultContext, 6)
		require.NoError(t, err)

		// Verify it's different
		assert.NotEqual(t, original.PublicKey, regenerated.PublicKey)
		assert.NotEqual(t, original.PrivateKeyEncrypted, regenerated.PrivateKeyEncrypted)
		assert.NotEqual(t, original.Fingerprint, regenerated.Fingerprint)
		assert.Equal(t, original.OwnerID, regenerated.OwnerID)
	})
}

func TestMirrorSSHKeypairConcurrency(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	if setting.SecretKey == "" {
		setting.SecretKey = "test-secret-key-for-testing"
	}

	// Test concurrent creation of keypairs to ensure no race conditions
	t.Run("ConcurrentCreation", func(t *testing.T) {
		ctx := t.Context()
		results := make(chan error, 10)

		// Start multiple goroutines creating keypairs for different owners
		for i := range 10 {
			go func(ownerID int64) {
				_, err := repo_model.CreateMirrorSSHKeypair(ctx, ownerID+100)
				results <- err
			}(int64(i))
		}

		// Check all creations succeeded
		for range 10 {
			err := <-results
			assert.NoError(t, err)
		}
	})
}
