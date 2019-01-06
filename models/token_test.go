// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAccessToken(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	token := &AccessToken{
		UID:  3,
		Name: "Token C",
	}
	assert.NoError(t, NewAccessToken(token))
	AssertExistsAndLoadBean(t, token)

	invalidToken := &AccessToken{
		ID:   token.ID, // duplicate
		UID:  2,
		Name: "Token F",
	}
	assert.Error(t, NewAccessToken(invalidToken))
}

func TestGetAccessTokenBySHA(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	token, err := GetAccessTokenBySHA("hash1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)
	assert.Equal(t, "Token A", token.Name)
	assert.Equal(t, "hash1", token.Sha1)

	token, err = GetAccessTokenBySHA("notahash")
	assert.Error(t, err)
	assert.True(t, IsErrAccessTokenNotExist(err))

	token, err = GetAccessTokenBySHA("")
	assert.Error(t, err)
	assert.True(t, IsErrAccessTokenEmpty(err))
}

func TestListAccessTokens(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tokens, err := ListAccessTokens(1)
	assert.NoError(t, err)
	if assert.Len(t, tokens, 2) {
		assert.Equal(t, int64(1), tokens[0].UID)
		assert.Equal(t, int64(1), tokens[1].UID)
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token A")
		assert.Contains(t, []string{tokens[0].Name, tokens[1].Name}, "Token B")
	}

	tokens, err = ListAccessTokens(2)
	assert.NoError(t, err)
	if assert.Len(t, tokens, 1) {
		assert.Equal(t, int64(2), tokens[0].UID)
		assert.Equal(t, "Token A", tokens[0].Name)
	}

	tokens, err = ListAccessTokens(100)
	assert.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestUpdateAccessToken(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	token, err := GetAccessTokenBySHA("hash2")
	assert.NoError(t, err)
	token.Name = "Token Z"

	assert.NoError(t, UpdateAccessToken(token))
	AssertExistsAndLoadBean(t, token)
}

func TestDeleteAccessTokenByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	token, err := GetAccessTokenBySHA("hash2")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), token.UID)

	assert.NoError(t, DeleteAccessTokenByID(token.ID, 1))
	AssertNotExistsBean(t, token)

	err = DeleteAccessTokenByID(100, 100)
	assert.Error(t, err)
	assert.True(t, IsErrAccessTokenNotExist(err))
}
