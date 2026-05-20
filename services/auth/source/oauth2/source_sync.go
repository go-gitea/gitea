// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"errors"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// Sync causes this OAuth2 source to synchronize its users with the db.
func (source *Source) Sync(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers[%s] %d", source.AuthSource.Name, source.AuthSource.ID)

	if !updateExisting {
		log.Info("SyncExternalUsers[%s] not running since updateExisting is false", source.AuthSource.Name)
		return nil
	}

	provider, err := createProvider(source.AuthSource.Name, source)
	if err != nil {
		return err
	}

	if !provider.RefreshTokenAvailable() {
		log.Trace("SyncExternalUsers[%s] provider doesn't support refresh tokens, can't synchronize", source.AuthSource.Name)
		return nil
	}

	opts := user_model.FindExternalUserOptions{
		HasRefreshToken: true,
		Expired:         true,
		LoginSourceID:   source.AuthSource.ID,
	}

	return user_model.IterateExternalLogin(ctx, opts, func(ctx context.Context, u *user_model.ExternalLoginUser) error {
		return source.refresh(ctx, provider, u)
	})
}

func (source *Source) refresh(ctx context.Context, provider goth.Provider, u *user_model.ExternalLoginUser) error {
	log.Trace("Syncing login_source_id=%d external_id=%s expiration=%s", u.LoginSourceID, u.ExternalID, u.ExpiresAt)

	token, err := provider.RefreshToken(u.RefreshToken)
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if !errors.As(err, &retrieveErr) || retrieveErr.ErrorCode != "invalid_grant" {
			return err
		}
		log.Info("SyncExternalUsers[%s] dropping invalid refresh token for user %d", source.AuthSource.Name, u.UserID)

		// Refresh tokens can expire or be revoked independently from the
		// upstream account state. Keep the local user active and only clear
		// the cached tokens until the next successful OAuth sign-in updates them.
		u.AccessToken = ""
		u.RefreshToken = ""
		u.ExpiresAt = time.Time{}

		return user_model.UpdateExternalUserByExternalID(ctx, u)
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
