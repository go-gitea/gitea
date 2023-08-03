// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCheckAuthToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Empty", func(t *testing.T) {
		token, err := CheckAuthToken(db.DefaultContext, "")
		assert.NoError(t, err)
		assert.Nil(t, token)
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		token, err := CheckAuthToken(db.DefaultContext, "dummy")
		assert.ErrorIs(t, err, ErrAuthTokenInvalidFormat)
		assert.Nil(t, token)
	})

	t.Run("NotFound", func(t *testing.T) {
		token, err := CheckAuthToken(db.DefaultContext, "notexists:dummy")
		assert.ErrorIs(t, err, ErrAuthTokenExpired)
		assert.Nil(t, token)
	})

	t.Run("Expired", func(t *testing.T) {
		timeutil.Set(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

		at, token, err := CreateAuthTokenForUserID(db.DefaultContext, 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		timeutil.Unset()

		at2, err := CheckAuthToken(db.DefaultContext, at.ID+":"+token)
		assert.ErrorIs(t, err, ErrAuthTokenExpired)
		assert.Nil(t, at2)

		assert.NoError(t, auth_model.DeleteAuthTokenByID(db.DefaultContext, at.ID))
	})

	t.Run("InvalidHash", func(t *testing.T) {
		at, token, err := CreateAuthTokenForUserID(db.DefaultContext, 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		at2, err := CheckAuthToken(db.DefaultContext, at.ID+":"+token+"dummy")
		assert.ErrorIs(t, err, ErrAuthTokenInvalidHash)
		assert.Nil(t, at2)

		assert.NoError(t, auth_model.DeleteAuthTokenByID(db.DefaultContext, at.ID))
	})

	t.Run("Valid", func(t *testing.T) {
		at, token, err := CreateAuthTokenForUserID(db.DefaultContext, 2)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.NotEmpty(t, token)

		at2, err := CheckAuthToken(db.DefaultContext, at.ID+":"+token)
		assert.NoError(t, err)
		assert.NotNil(t, at2)

		assert.NoError(t, auth_model.DeleteAuthTokenByID(db.DefaultContext, at.ID))
	})
}

func TestRegenerateAuthToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	timeutil.Set(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.Unset()

	at, token, err := CreateAuthTokenForUserID(db.DefaultContext, 2)
	assert.NoError(t, err)
	assert.NotNil(t, at)
	assert.NotEmpty(t, token)

	timeutil.Set(time.Date(2023, 1, 1, 0, 0, 1, 0, time.UTC))

	at2, token2, err := RegenerateAuthToken(db.DefaultContext, at)
	assert.NoError(t, err)
	assert.NotNil(t, at2)
	assert.NotEmpty(t, token2)

	assert.Equal(t, at.ID, at2.ID)
	assert.Equal(t, at.UserID, at2.UserID)
	assert.NotEqual(t, token, token2)
	assert.NotEqual(t, at.ExpiresUnix, at2.ExpiresUnix)

	assert.NoError(t, auth_model.DeleteAuthTokenByID(db.DefaultContext, at.ID))
}
