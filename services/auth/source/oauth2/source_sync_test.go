// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestSource(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	source := &Source{
		Provider: "fake",
		authSource: &auth.Source{
			ID:            12,
			Type:          auth.OAuth2,
			Name:          "fake",
			IsActive:      true,
			IsSyncEnabled: true,
		},
	}

	user := &user_model.User{
		LoginName:   "external",
		LoginType:   auth.OAuth2,
		LoginSource: source.authSource.ID,
		Name:        "test",
		Email:       "external@example.com",
	}

	err := user_model.CreateUser(context.Background(), user, &user_model.Meta{}, &user_model.CreateUserOverwriteOptions{})
	assert.NoError(t, err)

	e := &user_model.ExternalLoginUser{
		ExternalID:    "external",
		UserID:        user.ID,
		LoginSourceID: user.LoginSource,
		RefreshToken:  "valid",
	}
	err = user_model.LinkExternalToUser(context.Background(), user, e)
	assert.NoError(t, err)

	provider, err := createProvider(source.authSource.Name, source)
	assert.NoError(t, err)

	t.Run("refresh", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			err := source.refresh(context.Background(), provider, e)
			assert.NoError(t, err)

			e := &user_model.ExternalLoginUser{
				ExternalID:    e.ExternalID,
				LoginSourceID: e.LoginSourceID,
			}

			ok, err := user_model.GetExternalLogin(context.Background(), e)
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, "refresh", e.RefreshToken)
			assert.Equal(t, "token", e.AccessToken)

			u, err := user_model.GetUserByID(context.Background(), user.ID)
			assert.NoError(t, err)
			assert.True(t, u.IsActive)
		})

		t.Run("expired", func(t *testing.T) {
			err := source.refresh(context.Background(), provider, &user_model.ExternalLoginUser{
				ExternalID:    "external",
				UserID:        user.ID,
				LoginSourceID: user.LoginSource,
				RefreshToken:  "expired",
			})
			assert.NoError(t, err)

			e := &user_model.ExternalLoginUser{
				ExternalID:    e.ExternalID,
				LoginSourceID: e.LoginSourceID,
			}

			ok, err := user_model.GetExternalLogin(context.Background(), e)
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, "", e.RefreshToken)
			assert.Equal(t, "", e.AccessToken)

			u, err := user_model.GetUserByID(context.Background(), user.ID)
			assert.NoError(t, err)
			assert.False(t, u.IsActive)
		})
	})
}
