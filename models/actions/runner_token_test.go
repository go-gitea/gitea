// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	expectedToken, err := GetLatestRunnerToken(t.Context(), 1, 0)
	assert.NoError(t, err)
	assert.Equal(t, expectedToken, token)
}

func TestNewRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := NewRunnerToken(t.Context(), 1, 0)
	assert.NoError(t, err)
	expectedToken, err := GetLatestRunnerToken(t.Context(), 1, 0)
	assert.NoError(t, err)
	assert.Equal(t, expectedToken, token)
}

func TestNewRunnerTokenInvalidatesOldTokens(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// old versions stored repo level tokens together with the ID of the repository owner
	legacyRepoToken := &ActionRunnerToken{Token: "legacy-repo-token", OwnerID: 2, RepoID: 1, IsActive: true}
	assert.NoError(t, db.Insert(t.Context(), legacyRepoToken))
	otherRepoToken := &ActionRunnerToken{Token: "other-repo-token", RepoID: 2, IsActive: true}
	assert.NoError(t, db.Insert(t.Context(), otherRepoToken))
	ownerToken := &ActionRunnerToken{Token: "owner-token", OwnerID: 2, IsActive: true}
	assert.NoError(t, db.Insert(t.Context(), ownerToken))

	isActive := func(id int64) bool {
		return unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: id}).IsActive
	}

	newRepoToken, err := NewRunnerToken(t.Context(), 0, 1)
	assert.NoError(t, err)
	assert.True(t, isActive(newRepoToken.ID))
	assert.False(t, isActive(4), "the previous repo level token must be invalidated")
	assert.False(t, isActive(legacyRepoToken.ID), "a repo level token stored with an owner ID must be invalidated")
	assert.True(t, isActive(otherRepoToken.ID))
	assert.True(t, isActive(ownerToken.ID))
	assert.True(t, isActive(1))

	newOwnerToken, err := NewRunnerToken(t.Context(), 2, 0)
	assert.NoError(t, err)
	assert.True(t, isActive(newOwnerToken.ID))
	assert.False(t, isActive(ownerToken.ID))
	assert.True(t, isActive(newRepoToken.ID), "an owner level reset must not invalidate repo level tokens")
	assert.True(t, isActive(otherRepoToken.ID))
	assert.True(t, isActive(1))
}
