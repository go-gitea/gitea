// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"crypto/ed25519"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHKeypair(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("CreateSSHKeypair", func(t *testing.T) {
		// Test creating a new SSH keypair for a user
		keypair, err := user_model.CreateSSHKeypair(t.Context(), 1)
		require.NoError(t, err)
		assert.NotNil(t, keypair)
		assert.Equal(t, int64(1), keypair.OwnerID)
		assert.NotEmpty(t, keypair.PublicKey)
		assert.NotEmpty(t, keypair.PrivateKeyEncrypted)
		assert.NotEmpty(t, keypair.Fingerprint)

		// Verify the public key is in SSH format
		assert.Contains(t, keypair.PublicKey, "ssh-ed25519")

		// Test creating a keypair for an organization
		orgKeypair, err := user_model.CreateSSHKeypair(t.Context(), 2)
		require.NoError(t, err)
		assert.NotNil(t, orgKeypair)
		assert.Equal(t, int64(2), orgKeypair.OwnerID)

		// Ensure different owners get different keypairs
		assert.NotEqual(t, keypair.PublicKey, orgKeypair.PublicKey)
		assert.NotEqual(t, keypair.Fingerprint, orgKeypair.Fingerprint)
	})

	t.Run("GetSSHKeypairByOwner", func(t *testing.T) {
		// Create a keypair first
		created, err := user_model.CreateSSHKeypair(t.Context(), 3)
		require.NoError(t, err)

		// Test retrieving the keypair
		retrieved, err := user_model.GetSSHKeypairByOwner(t.Context(), 3)
		require.NoError(t, err)
		assert.Equal(t, created.OwnerID, retrieved.OwnerID)
		assert.Equal(t, created.PublicKey, retrieved.PublicKey)
		assert.Equal(t, created.Fingerprint, retrieved.Fingerprint)

		// Test retrieving non-existent keypair
		_, err = user_model.GetSSHKeypairByOwner(t.Context(), 999)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("GetDecryptedPrivateKey", func(t *testing.T) {
		// Ensure we have a valid SECRET_KEY for testing
		if setting.SecretKey == "" {
			setting.SecretKey = "test-secret-key-for-testing"
		}

		// Create a keypair
		keypair, err := user_model.CreateSSHKeypair(t.Context(), 4)
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

	t.Run("DeleteSSHKeypair", func(t *testing.T) {
		// Create a keypair
		_, err := user_model.CreateSSHKeypair(t.Context(), 5)
		require.NoError(t, err)

		// Verify it exists
		_, err = user_model.GetSSHKeypairByOwner(t.Context(), 5)
		require.NoError(t, err)

		// Delete it
		err = user_model.DeleteSSHKeypair(t.Context(), 5)
		require.NoError(t, err)

		// Verify it's gone
		_, err = user_model.GetSSHKeypairByOwner(t.Context(), 5)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("RegenerateSSHKeypair", func(t *testing.T) {
		// Create initial keypair
		original, err := user_model.CreateSSHKeypair(t.Context(), 6)
		require.NoError(t, err)

		// Regenerate it
		regenerated, err := user_model.RegenerateSSHKeypair(t.Context(), 6)
		require.NoError(t, err)

		// Verify it's different
		assert.NotEqual(t, original.PublicKey, regenerated.PublicKey)
		assert.NotEqual(t, original.PrivateKeyEncrypted, regenerated.PrivateKeyEncrypted)
		assert.NotEqual(t, original.Fingerprint, regenerated.Fingerprint)
		assert.Equal(t, original.OwnerID, regenerated.OwnerID)
	})
}

func TestSSHKeypairConcurrency(t *testing.T) {
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
				_, err := user_model.CreateSSHKeypair(ctx, ownerID+100)
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
