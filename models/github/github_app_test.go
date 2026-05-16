// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package github_test

import (
	"testing"

	github_model "code.gitea.io/gitea/models/github"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasRecentActivity(t *testing.T) {
	now := timeutil.TimeStampNow()

	tests := []struct {
		name         string
		lastUsedUnix timeutil.TimeStamp
		expectRecent bool
	}{
		{
			name:         "Never used",
			lastUsedUnix: 0,
			expectRecent: false,
		},
		{
			name:         "Used 1 day ago",
			lastUsedUnix: now - 1*24*3600,
			expectRecent: true,
		},
		{
			name:         "Used 6 days ago",
			lastUsedUnix: now - 6*24*3600,
			expectRecent: true,
		},
		{
			name:         "Used 8 days ago",
			lastUsedUnix: now - 8*24*3600,
			expectRecent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &github_model.AppCredential{
				LastUsedUnix: tt.lastUsedUnix,
			}
			assert.Equal(t, tt.expectRecent, cred.HasRecentActivity())
		})
	}
}

func TestHasUsed(t *testing.T) {
	tests := []struct {
		name         string
		lastUsedUnix timeutil.TimeStamp
		expectUsed   bool
	}{
		{
			name:         "Never used",
			lastUsedUnix: 0,
			expectUsed:   false,
		},
		{
			name:         "Used once",
			lastUsedUnix: 1700000000,
			expectUsed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &github_model.AppCredential{
				LastUsedUnix: tt.lastUsedUnix,
			}
			assert.Equal(t, tt.expectUsed, cred.HasUsed())
		})
	}
}

func TestTableName(t *testing.T) {
	cred := &github_model.AppCredential{}
	assert.Equal(t, "github_app_credential", cred.TableName())
}

func TestCreateGithubAppCredential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	cred := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "New Test App",
		ClientID:            "Iv1.newtest",
		InstallationID:      99999,
		PrivateKeyEncrypted: "new_encrypted_key",
		BaseURL:             "https://api.github.com",
	}

	err := github_model.CreateGithubAppCredential(t.Context(), cred)
	require.NoError(t, err)
	assert.NotZero(t, cred.ID)

	// Verify it was created
	loaded, err := github_model.GetGithubAppCredentialByID(t.Context(), cred.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Test App", loaded.Name)
	assert.Equal(t, "Iv1.newtest", loaded.ClientID)
}

func TestGetGithubAppCredentialByID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a test credential
	cred := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "Test GitHub App",
		ClientID:            "Iv1.test123",
		InstallationID:      12345,
		PrivateKeyEncrypted: "encrypted_key_data",
		BaseURL:             "https://api.github.com",
	}
	err := github_model.CreateGithubAppCredential(t.Context(), cred)
	require.NoError(t, err)

	// Get existing credential
	loaded, err := github_model.GetGithubAppCredentialByID(t.Context(), cred.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test GitHub App", loaded.Name)
	assert.Equal(t, "Iv1.test123", loaded.ClientID)
	assert.Equal(t, int64(12345), loaded.InstallationID)

	// Get non-existing credential
	_, err = github_model.GetGithubAppCredentialByID(t.Context(), 999999)
	require.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestGetGithubAppCredentialsByOwnerID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create test credentials for owner 1
	cred1 := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "Test GitHub App",
		ClientID:            "Iv1.test123",
		InstallationID:      12345,
		PrivateKeyEncrypted: "encrypted_key_data",
		BaseURL:             "https://api.github.com",
	}
	err := github_model.CreateGithubAppCredential(t.Context(), cred1)
	require.NoError(t, err)

	cred3 := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "Recently Used App",
		ClientID:            "Iv1.recent",
		InstallationID:      11111,
		PrivateKeyEncrypted: "recent_key",
		BaseURL:             "https://api.github.com",
	}
	err = github_model.CreateGithubAppCredential(t.Context(), cred3)
	require.NoError(t, err)

	// Create test credential for owner 2
	cred2 := &github_model.AppCredential{
		OwnerID:             2,
		Name:                "Another GitHub App",
		ClientID:            "Iv1.test456",
		InstallationID:      67890,
		PrivateKeyEncrypted: "another_encrypted_key",
		BaseURL:             "https://github.example.com/api/v3",
	}
	err = github_model.CreateGithubAppCredential(t.Context(), cred2)
	require.NoError(t, err)

	// Owner with multiple credentials
	creds, err := github_model.GetGithubAppCredentialsByOwnerID(t.Context(), 1)
	require.NoError(t, err)
	assert.Len(t, creds, 2)

	// Owner with one credential
	creds, err = github_model.GetGithubAppCredentialsByOwnerID(t.Context(), 2)
	require.NoError(t, err)
	assert.Len(t, creds, 1)
	assert.Equal(t, "Another GitHub App", creds[0].Name)

	// Owner with no credentials
	creds, err = github_model.GetGithubAppCredentialsByOwnerID(t.Context(), 999)
	require.NoError(t, err)
	assert.Empty(t, creds)
}

func TestUpdateGithubAppCredentialLastUsed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a test credential with last_used_unix = 0
	cred := &github_model.AppCredential{
		OwnerID:             2,
		Name:                "Another GitHub App",
		ClientID:            "Iv1.test456",
		InstallationID:      67890,
		PrivateKeyEncrypted: "another_encrypted_key",
		BaseURL:             "https://github.example.com/api/v3",
		LastUsedUnix:        0,
	}
	err := github_model.CreateGithubAppCredential(t.Context(), cred)
	require.NoError(t, err)

	// Verify initial state
	assert.Equal(t, timeutil.TimeStamp(0), cred.LastUsedUnix)

	// Update last used
	err = github_model.UpdateGithubAppCredentialLastUsed(t.Context(), cred.ID)
	require.NoError(t, err)

	// Verify it was updated
	updated := unittest.AssertExistsAndLoadBean(t, &github_model.AppCredential{ID: cred.ID})
	assert.Greater(t, updated.LastUsedUnix, timeutil.TimeStamp(0))
}

func TestDeleteGithubAppCredential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a test credential
	cred := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "Test GitHub App",
		ClientID:            "Iv1.test123",
		InstallationID:      12345,
		PrivateKeyEncrypted: "encrypted_key_data",
		BaseURL:             "https://api.github.com",
	}
	err := github_model.CreateGithubAppCredential(t.Context(), cred)
	require.NoError(t, err)

	// Verify it exists
	unittest.AssertExistsAndLoadBean(t, &github_model.AppCredential{ID: cred.ID})

	// Delete it
	err = github_model.DeleteGithubAppCredential(t.Context(), cred.ID)
	require.NoError(t, err)

	// Verify it's gone
	unittest.AssertNotExistsBean(t, &github_model.AppCredential{ID: cred.ID})
}

func TestCheckGithubAppCredentialOwnership(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a test credential owned by user 1
	cred := &github_model.AppCredential{
		OwnerID:             1,
		Name:                "Test GitHub App",
		ClientID:            "Iv1.test123",
		InstallationID:      12345,
		PrivateKeyEncrypted: "encrypted_key_data",
		BaseURL:             "https://api.github.com",
	}
	err := github_model.CreateGithubAppCredential(t.Context(), cred)
	require.NoError(t, err)

	// Owner matches
	owns, err := github_model.CheckGithubAppCredentialOwnership(t.Context(), cred.ID, 1)
	require.NoError(t, err)
	assert.True(t, owns)

	// Owner doesn't match
	owns, err = github_model.CheckGithubAppCredentialOwnership(t.Context(), cred.ID, 2)
	require.NoError(t, err)
	assert.False(t, owns)

	// Non-existent credential
	owns, err = github_model.CheckGithubAppCredentialOwnership(t.Context(), 999999, 1)
	require.NoError(t, err)
	assert.False(t, owns)
}
