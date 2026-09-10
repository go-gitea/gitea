// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/hex"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestCheckAuthToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Empty", func(t *testing.T) {
		token, err := CheckAuthToken(t.Context(), "")
		assert.NoError(t, err)
		assert.Nil(t, token)
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		token, err := CheckAuthToken(t.Context(), "dummy")
		assert.ErrorIs(t, err, ErrAuthTokenInvalidFormat)
		assert.Nil(t, token)
	})

	t.Run("NotFound", func(t *testing.T) {
		token, err := CheckAuthToken(t.Context(), "notexists:dummy")
		assert.ErrorIs(t, err, ErrAuthTokenExpired)
		assert.Nil(t, token)
	})

	t.Run("Expired", func(t *testing.T) {
		timeutil.MockSet(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

		at, token, err := CreateAuthTokenForUserID(t.Context(), 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		timeutil.MockUnset()

		at2, err := CheckAuthToken(t.Context(), at.ID+":"+token)
		assert.ErrorIs(t, err, ErrAuthTokenExpired)
		assert.Nil(t, at2)

		assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at.ID))
	})

	t.Run("InvalidHash", func(t *testing.T) {
		at, token, err := CreateAuthTokenForUserID(t.Context(), 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		at2, err := CheckAuthToken(t.Context(), at.ID+":"+token+"dummy")
		assert.ErrorIs(t, err, ErrAuthTokenInvalidHash)
		assert.Nil(t, at2)

		// a hash mismatch signals a compromised token, which must be revoked
		_, err = auth_model.GetAuthTokenByID(t.Context(), at.ID)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("Valid", func(t *testing.T) {
		at, token, err := CreateAuthTokenForUserID(t.Context(), 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		at2, err := CheckAuthToken(t.Context(), at.ID+":"+token)
		assert.NoError(t, err)
		assert.NotNil(t, at2)

		assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at.ID))
	})
}

func TestRegenerateAuthToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	timeutil.MockSet(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	at, token, err := CreateAuthTokenForUserID(t.Context(), 2)
	assert.NoError(t, err)
	assert.NotNil(t, at)
	assert.NotEmpty(t, token)

	timeutil.MockSet(time.Date(2023, 1, 1, 0, 0, 1, 0, time.UTC))

	at2, token2, err := RegenerateAuthToken(t.Context(), at)
	assert.NoError(t, err)
	assert.NotNil(t, at2)
	assert.NotEmpty(t, token2)

	assert.Equal(t, at.ID, at2.ID)
	assert.Equal(t, at.UserID, at2.UserID)
	assert.NotEqual(t, token, token2)
	assert.NotEqual(t, at.ExpiresUnix, at2.ExpiresUnix)

	assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at.ID))
}

func TestCheckAuthTokens(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	newToken := func(t *testing.T, uid int64) (*auth_model.AuthToken, string) {
		t.Helper()
		at, token, err := CreateAuthTokenForUserID(t.Context(), uid)
		assert.NoError(t, err)
		return at, at.ID + ":" + token
	}

	t.Run("Legacy", func(t *testing.T) {
		at, value := newToken(t, 2)
		defer func() { assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at.ID)) }()

		checked, err := CheckAuthTokens(t.Context(), value)
		assert.NoError(t, err)
		assert.False(t, checked.Compromised)
		assert.Len(t, checked.Valid, 1)
	})

	t.Run("KeepsSurvivors", func(t *testing.T) {
		at1, value1 := newToken(t, 2)
		at2, value2 := newToken(t, 4)
		defer func() {
			assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at1.ID))
			assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at2.ID))
		}()

		checked, err := CheckAuthTokens(t.Context(), value1+",notexists:dummy,"+value2)
		assert.NoError(t, err)
		assert.False(t, checked.Compromised)
		assert.Len(t, checked.Valid, 2)
	})

	t.Run("CompromisedDoesNotDropOthers", func(t *testing.T) {
		at1, value1 := newToken(t, 2)
		at2, _ := newToken(t, 4)
		defer func() { assert.NoError(t, auth_model.DeleteAuthTokenByID(t.Context(), at1.ID)) }()

		checked, err := CheckAuthTokens(t.Context(), at2.ID+":"+hex.EncodeToString([]byte("wrong"))+","+value1)
		assert.NoError(t, err)
		assert.True(t, checked.Compromised)
		assert.Len(t, checked.Valid, 1)
		assert.Equal(t, at1.ID, checked.Valid[0].ID)

		// a mismatching hash revokes the token it was presented for
		_, err = auth_model.GetAuthTokenByID(t.Context(), at2.ID)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})
}
