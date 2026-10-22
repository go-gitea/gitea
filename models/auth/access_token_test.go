// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestNewAccessToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := &auth_model.AccessToken{
		UID:  3,
		Name: "Token C",
	}
	assert.NoError(t, auth_model.NewAccessToken(t.Context(), token))
	unittest.AssertExistsAndLoadBean(t, token)

	invalidToken := &auth_model.AccessToken{
		ID:   token.ID, // duplicate
		UID:  2,
		Name: "Token F",
	}
	assert.Error(t, auth_model.NewAccessToken(t.Context(), invalidToken))
}

func TestAccessTokenByNameExists(t *testing.T) {
	name := "Token Gitea"

	assert.NoError(t, unittest.PrepareTestDatabase())
	token := &auth_model.AccessToken{
		UID:  3,
		Name: name,
	}

	// Check to make sure it doesn't exists already
	exist, err := auth_model.AccessTokenByNameExists(t.Context(), token)
	assert.NoError(t, err)
	assert.False(t, exist)

	// Save it to the database
	assert.NoError(t, auth_model.NewAccessToken(t.Context(), token))
	unittest.AssertExistsAndLoadBean(t, token)

	// This token must be found by name in the DB now
	exist, err = auth_model.AccessTokenByNameExists(t.Context(), token)
	assert.NoError(t, err)
	assert.True(t, exist)

	user4Token := &auth_model.AccessToken{
		UID:  4,
		Name: name,
	}

	// Name matches but different user ID, this shouldn't exists in the
	// database
	exist, err = auth_model.AccessTokenByNameExists(t.Context(), user4Token)
	assert.NoError(t, err)
	assert.False(t, exist)
}

func TestGetAccessTokenBySHA(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := auth_model.GetAccessTokenBySHA(t.Context(), "d2c6c1ba3890b309189a8e618c72a162e4efbf36")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)
	assert.Equal(t, "Token A", token.Name)
	assert.Equal(t, "2b3668e11cb82d3af8c6e4524fc7841297668f5008d1626f0ad3417e9fa39af84c268248b78c481daa7e5dc437784003494f", token.TokenHash)
	assert.Equal(t, "e4efbf36", token.TokenLastEight)

	_, err = auth_model.GetAccessTokenBySHA(t.Context(), "notahash")
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)

	_, err = auth_model.GetAccessTokenBySHA(t.Context(), "")
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestListAccessTokens(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	tokens, err := db.Find[auth_model.AccessToken](t.Context(), auth_model.ListAccessTokensOptions{UserID: 1})
	assert.NoError(t, err)
	if assert.Len(t, tokens, 2) {
		assert.Equal(t, int64(1), tokens[0].UID)
		assert.Equal(t, int64(1), tokens[1].UID)
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token A")
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token B")
	}

	tokens, err = db.Find[auth_model.AccessToken](t.Context(), auth_model.ListAccessTokensOptions{UserID: 2})
	assert.NoError(t, err)
	if assert.Len(t, tokens, 1) {
		assert.Equal(t, int64(2), tokens[0].UID)
		assert.Equal(t, "Token A", tokens[0].Name)
	}

	tokens, err = db.Find[auth_model.AccessToken](t.Context(), auth_model.ListAccessTokensOptions{UserID: 100})
	assert.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestTouchAccessToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const sha = "4c6f36e6cf498e2a448662f915d932c09c5a146c"
	interval := int64(auth_model.AccessTokenTouchInterval / time.Second)
	now := func() int64 { return int64(timeutil.TimeStampNow()) }
	load := func(id int64) *auth_model.AccessToken {
		return unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{ID: id})
	}
	setUpdated := func(id, created, updated int64) {
		_, err := db.GetEngine(t.Context()).Exec("UPDATE access_token SET created_unix = ?, updated_unix = ? WHERE id = ?", created, updated, id)
		assert.NoError(t, err)
	}

	token, err := auth_model.GetAccessTokenBySHA(t.Context(), sha)
	assert.NoError(t, err)

	// a stale token is touched, and only updated_unix is written
	setUpdated(token.ID, now()-interval-100, now()-interval-10)
	token = load(token.ID)
	oldName := token.Name
	token.Name = "must not be persisted"
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), token))
	assert.GreaterOrEqual(t, int64(token.UpdatedUnix), now()-5)
	reloaded := load(token.ID)
	assert.Equal(t, token.UpdatedUnix, reloaded.UpdatedUnix)
	assert.Equal(t, oldName, reloaded.Name)

	// a recently used token is not written again
	recent := now() - 1
	setUpdated(token.ID, now()-interval-100, recent)
	reloaded = load(token.ID)
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), reloaded))
	assert.Equal(t, recent, int64(load(token.ID).UpdatedUnix))

	// the first use shortly after creation is recorded, so the token is shown as used
	created := now() - 10
	setUpdated(token.ID, created, created)
	fresh := load(token.ID)
	assert.False(t, fresh.HasUsed)
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), fresh))
	assert.Greater(t, int64(load(token.ID).UpdatedUnix), created)
	assert.True(t, load(token.ID).HasUsed)

	// concurrent requests hold separate stale copies of the same row: only the first one writes
	setUpdated(token.ID, now()-interval-100, now()-interval-10)
	first, second := load(token.ID), load(token.ID)
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), first))
	written := int64(load(token.ID).UpdatedUnix)
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), second))
	assert.Equal(t, written, int64(load(token.ID).UpdatedUnix))
	assert.Less(t, int64(second.UpdatedUnix), written, "the stale copy is not refreshed when it did not write")

	// a delayed request never overwrites a newer timestamp with an older one
	newer := now() + 5
	setUpdated(token.ID, now()-interval-100, newer)
	delayed := load(token.ID)
	delayed.UpdatedUnix = timeutil.TimeStamp(now() - interval - 10)
	assert.NoError(t, auth_model.TouchAccessToken(t.Context(), delayed))
	assert.Equal(t, newer, int64(load(token.ID).UpdatedUnix))
}

func TestRegenerateAccessToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const oldToken = "d2c6c1ba3890b309189a8e618c72a162e4efbf36"

	// prime the successful-lookup cache with the old token value, as a real request would
	before, err := auth_model.GetAccessTokenBySHA(t.Context(), oldToken)
	assert.NoError(t, err)
	assert.Equal(t, "Token A", before.Name)

	regenerated, err := auth_model.RegenerateAccessToken(t.Context(), before.ID, before.UID)
	assert.NoError(t, err)
	assert.Equal(t, before.ID, regenerated.ID)
	assert.Equal(t, before.Name, regenerated.Name)
	assert.Equal(t, before.Scope, regenerated.Scope)
	assert.NotEqual(t, before.TokenHash, regenerated.TokenHash)
	assert.NotEmpty(t, regenerated.Token)

	// the old token value must stop authenticating, even though it was cached as successful above
	_, err = auth_model.GetAccessTokenBySHA(t.Context(), oldToken)
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)

	// the new token value must authenticate
	found, err := auth_model.GetAccessTokenBySHA(t.Context(), regenerated.Token)
	assert.NoError(t, err)
	assert.Equal(t, before.ID, found.ID)
	assert.Equal(t, before.UpdatedUnix, found.UpdatedUnix)

	// wrong owner
	_, err = auth_model.RegenerateAccessToken(t.Context(), before.ID, before.UID+1)
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)

	// nonexistent token
	_, err = auth_model.RegenerateAccessToken(t.Context(), 100, 100)
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestDeleteAccessTokenByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	token, err := auth_model.GetAccessTokenBySHA(t.Context(), "4c6f36e6cf498e2a448662f915d932c09c5a146c")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)

	assert.NoError(t, auth_model.DeleteAccessTokenByID(t.Context(), token.ID, 1))
	unittest.AssertNotExistsBean(t, token)

	err = auth_model.DeleteAccessTokenByID(t.Context(), 100, 100)
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrNotExist)
}
