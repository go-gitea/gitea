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
			cred := &github_model.GithubAppCredential{
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
			cred := &github_model.GithubAppCredential{
				LastUsedUnix: tt.lastUsedUnix,
			}
			assert.Equal(t, tt.expectUsed, cred.HasUsed())
		})
	}
}

func TestTableName(t *testing.T) {
	cred := &github_model.GithubAppCredential{}
	assert.Equal(t, "github_app_credential", cred.TableName())
}

func TestCreateGithubAppCredential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	cred := &github_model.GithubAppCredential{
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

	// Get existing credential
	cred, err := github_model.GetGithubAppCredentialByID(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, "Test GitHub App", cred.Name)
	assert.Equal(t, "Iv1.test123", cred.ClientID)
	assert.Equal(t, int64(12345), cred.InstallationID)

	// Get non-existing credential
	_, err = github_model.GetGithubAppCredentialByID(t.Context(), 999999)
	require.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestGetGithubAppCredentialsByOwnerID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Owner with multiple credentials
	creds, err := github_model.GetGithubAppCredentialsByOwnerID(t.Context(), 1)
	require.NoError(t, err)
	assert.Len(t, creds, 2) // IDs 1 and 3

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

	// Get initial state
	cred := unittest.AssertExistsAndLoadBean(t, &github_model.GithubAppCredential{ID: 2})
	assert.Equal(t, timeutil.TimeStamp(0), cred.LastUsedUnix)

	// Update last used
	err := github_model.UpdateGithubAppCredentialLastUsed(t.Context(), 2)
	require.NoError(t, err)

	// Verify it was updated
	updated := unittest.AssertExistsAndLoadBean(t, &github_model.GithubAppCredential{ID: 2})
	assert.Greater(t, updated.LastUsedUnix, timeutil.TimeStamp(0))
}

func TestDeleteGithubAppCredential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Verify it exists
	unittest.AssertExistsAndLoadBean(t, &github_model.GithubAppCredential{ID: 1})

	// Delete it
	err := github_model.DeleteGithubAppCredential(t.Context(), 1)
	require.NoError(t, err)

	// Verify it's gone
	unittest.AssertNotExistsBean(t, &github_model.GithubAppCredential{ID: 1})
}

func TestCheckGithubAppCredentialOwnership(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Owner matches
	owns, err := github_model.CheckGithubAppCredentialOwnership(t.Context(), 1, 1)
	require.NoError(t, err)
	assert.True(t, owns)

	// Owner doesn't match
	owns, err = github_model.CheckGithubAppCredentialOwnership(t.Context(), 1, 2)
	require.NoError(t, err)
	assert.False(t, owns)

	// Non-existent credential
	owns, err = github_model.CheckGithubAppCredentialOwnership(t.Context(), 999999, 1)
	require.NoError(t, err)
	assert.False(t, owns)
}
