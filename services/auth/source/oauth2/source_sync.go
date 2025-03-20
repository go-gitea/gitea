// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// Sync causes this OAuth2 source to synchronize its users with the db.
func (source *Source) Sync(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers[%s] %d", source.authSource.Name, source.authSource.ID)

	if !updateExisting {
		log.Info("SyncExternalUsers[%s] not running since updateExisting is false", source.authSource.Name)
		return nil
	}

	provider, err := createProvider(source.authSource.Name, source)
	if err != nil {
		return err
	}

	if !provider.RefreshTokenAvailable() {
		log.Trace("SyncExternalUsers[%s] provider doesn't support refresh tokens, can't synchronize", source.authSource.Name)
		return nil
	}

	opts := user_model.FindExternalUserOptions{
		HasRefreshToken: true,
		Expired:         true,
		LoginSourceID:   source.authSource.ID,
	}

	return user_model.IterateExternalLogin(ctx, opts, func(ctx context.Context, u *user_model.ExternalLoginUser) error {
		return source.refresh(ctx, provider, u)
	})
}

func (source *Source) refresh(ctx context.Context, provider goth.Provider, u *user_model.ExternalLoginUser) error {
	log.Trace("Syncing login_source_id=%d external_id=%s expiration=%s", u.LoginSourceID, u.ExternalID, u.ExpiresAt)

	shouldDisable := false

	token, err := provider.RefreshToken(u.RefreshToken)
	if err != nil {
		if err, ok := err.(*oauth2.RetrieveError); ok && err.ErrorCode == "invalid_grant" {
			// this signals that the token is not valid and the user should be disabled
			shouldDisable = true
		} else {
			return err
		}
	}

	user := &user_model.User{
		LoginName:   u.ExternalID,
		LoginType:   auth.OAuth2,
		LoginSource: u.LoginSourceID,
	}

	hasUser, err := user_model.GetUser(ctx, user)
	if err != nil {
		return err
	}

	// If the grant is no longer valid, disable the user and
	// delete local tokens. If the OAuth2 provider still
	// recognizes them as a valid user, they will be able to login
	// via their provider and reactivate their account.
	if shouldDisable {
		log.Info("SyncExternalUsers[%s] disabling user %d", source.authSource.Name, user.ID)

		return db.WithTx(ctx, func(ctx context.Context) error {
			if hasUser {
				user.IsActive = false
				err := user_model.UpdateUserCols(ctx, user, "is_active")
				if err != nil {
					return err
				}
			}

			// Delete stored tokens, since they are invalid. This
			// also provents us from checking this in subsequent runs.
			u.AccessToken = ""
			u.RefreshToken = ""
			u.ExpiresAt = time.Time{}

			return user_model.UpdateExternalUserByExternalID(ctx, u)
		})
	}

	// Otherwise, update the tokens
	u.AccessToken = token.AccessToken
	u.ExpiresAt = token.Expiry

	// Some providers only update access tokens provide a new
	// refresh token, so avoid updating it if it's empty
	if token.RefreshToken != "" {
		u.RefreshToken = token.RefreshToken
	}

	err = user_model.UpdateExternalUserByExternalID(ctx, u)

	return err
}
