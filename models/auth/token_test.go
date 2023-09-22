// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestNewAccessToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := &auth_model.AccessToken{
		UID:  3,
		Name: "Token C",
	}
	assert.NoError(t, auth_model.NewAccessToken(db.DefaultContext, token))
	unittest.AssertExistsAndLoadBean(t, token)

	invalidToken := &auth_model.AccessToken{
		ID:   token.ID, // duplicate
		UID:  2,
		Name: "Token F",
	}
	assert.Error(t, auth_model.NewAccessToken(db.DefaultContext, invalidToken))
}

func TestAccessTokenByNameExists(t *testing.T) {
	name := "Token Gitea"

	assert.NoError(t, unittest.PrepareTestDatabase())
	token := &auth_model.AccessToken{
		UID:  3,
		Name: name,
	}

	// Check to make sure it doesn't exists already
	exist, err := auth_model.AccessTokenByNameExists(db.DefaultContext, token)
	assert.NoError(t, err)
	assert.False(t, exist)

	// Save it to the database
	assert.NoError(t, auth_model.NewAccessToken(db.DefaultContext, token))
	unittest.AssertExistsAndLoadBean(t, token)

	// This token must be found by name in the DB now
	exist, err = auth_model.AccessTokenByNameExists(db.DefaultContext, token)
	assert.NoError(t, err)
	assert.True(t, exist)

	user4Token := &auth_model.AccessToken{
		UID:  4,
		Name: name,
	}

	// Name matches but different user ID, this shouldn't exists in the
	// database
	exist, err = auth_model.AccessTokenByNameExists(db.DefaultContext, user4Token)
	assert.NoError(t, err)
	assert.False(t, exist)
}

func TestGetAccessTokenBySHA(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := auth_model.GetAccessTokenBySHA(db.DefaultContext, "d2c6c1ba3890b309189a8e618c72a162e4efbf36")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)
	assert.Equal(t, "Token A", token.Name)
	assert.Equal(t, "2b3668e11cb82d3af8c6e4524fc7841297668f5008d1626f0ad3417e9fa39af84c268248b78c481daa7e5dc437784003494f", token.TokenHash)
	assert.Equal(t, "e4efbf36", token.TokenLastEight)

	_, err = auth_model.GetAccessTokenBySHA(db.DefaultContext, "notahash")
	assert.Error(t, err)
	assert.True(t, auth_model.IsErrAccessTokenNotExist(err))

	_, err = auth_model.GetAccessTokenBySHA(db.DefaultContext, "")
	assert.Error(t, err)
	assert.True(t, auth_model.IsErrAccessTokenEmpty(err))
}

func TestListAccessTokens(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	tokens, err := auth_model.ListAccessTokens(db.DefaultContext, auth_model.ListAccessTokensOptions{UserID: 1})
	assert.NoError(t, err)
	if assert.Len(t, tokens, 2) {
		assert.Equal(t, int64(1), tokens[0].UID)
		assert.Equal(t, int64(1), tokens[1].UID)
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token A")
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token B")
	}

	tokens, err = auth_model.ListAccessTokens(db.DefaultContext, auth_model.ListAccessTokensOptions{UserID: 2})
	assert.NoError(t, err)
	if assert.Len(t, tokens, 1) {
		assert.Equal(t, int64(2), tokens[0].UID)
		assert.Equal(t, "Token A", tokens[0].Name)
	}

	tokens, err = auth_model.ListAccessTokens(db.DefaultContext, auth_model.ListAccessTokensOptions{UserID: 100})
	assert.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestUpdateAccessToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := auth_model.GetAccessTokenBySHA(db.DefaultContext, "4c6f36e6cf498e2a448662f915d932c09c5a146c")
	assert.NoError(t, err)
	token.Name = "Token Z"

	assert.NoError(t, auth_model.UpdateAccessToken(db.DefaultContext, token))
	unittest.AssertExistsAndLoadBean(t, token)
}

func TestDeleteAccessTokenByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	token, err := auth_model.GetAccessTokenBySHA(db.DefaultContext, "4c6f36e6cf498e2a448662f915d932c09c5a146c")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)

	assert.NoError(t, auth_model.DeleteAccessTokenByID(db.DefaultContext, token.ID, 1))
	unittest.AssertNotExistsBean(t, token)

	err = auth_model.DeleteAccessTokenByID(db.DefaultContext, 100, 100)
	assert.Error(t, err)
	assert.True(t, auth_model.IsErrAccessTokenNotExist(err))
}
