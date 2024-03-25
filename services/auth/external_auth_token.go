// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	auth "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/session"

	"github.com/markbates/goth"
)

func toExternalAuthToken(ctx context.Context, sessionID string, user *user_model.User, gothUser *goth.User) (*auth.ExternalAuthToken, error) {
	authSource, err := auth.GetActiveOAuth2SourceByName(ctx, gothUser.Provider)
	if err != nil {
		return nil, err
	}
	return &auth.ExternalAuthToken{
		SessionID:         sessionID,
		UserID:            user.ID,
		ExternalID:        gothUser.UserID,
		LoginSourceID:     authSource.ID,
		RawData:           gothUser.RawData,
		AccessToken:       gothUser.AccessToken,
		AccessTokenSecret: gothUser.AccessTokenSecret,
		RefreshToken:      gothUser.RefreshToken,
		ExpiresAt:         gothUser.ExpiresAt,
		IDToken:           gothUser.IDToken,
	}, nil
}

func SetExternalAuthToken(ctx context.Context, sessionID string, user *user_model.User, gothUser *goth.User) error {
	t, err := toExternalAuthToken(ctx, sessionID, user, gothUser)
	if err != nil {
		return err
	}

	oldt, err := auth.GetExternalAuthTokenBySessionID(ctx, sessionID)
	if auth.IsErrExternalAuthTokenNotExist(err) {
		return auth.InsertExternalAuthToken(ctx, t)
	} else if err != nil {
		return err
	}

	t.AuthTokenID = oldt.AuthTokenID
	return auth.UpdateExternalAuthTokenBySessionID(ctx, sessionID, t)
}

func UpdateExternalAuthTokenSessionID(ctx context.Context, oldSessionID, sessionID string) error {
	t, err := auth.GetExternalAuthTokenBySessionID(ctx, oldSessionID)
	if err != nil {
		return err
	}
	t.SessionID = sessionID
	return auth.UpdateExternalAuthTokenBySessionID(ctx, oldSessionID, t)
}

func UpdateExternalAuthTokenSessionIDByAuthTokenID(ctx context.Context, authTokenID, sessionID string) error {
	t, err := auth.GetExternalAuthTokenByAuthTokenID(ctx, authTokenID)
	if err != nil {
		return err
	}
	oldSessionID := t.SessionID
	t.SessionID = sessionID
	return auth.UpdateExternalAuthTokenBySessionID(ctx, oldSessionID, t)
}

func UpdateExternalAuthTokenAuthTokenID(ctx context.Context, sessionID, authTokenID string) error {
	t, err := auth.GetExternalAuthTokenBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	t.AuthTokenID = authTokenID
	return auth.UpdateExternalAuthTokenBySessionID(ctx, sessionID, t)
}

func CleanExternalAuthTokensByUser(ctx context.Context, userID int64) error {
	tokens, err := auth.GetExternalAuthTokenSessionIDsAndAuthTokenIDs(ctx, userID, 0)
	if err != nil {
		return err
	}
	sessionProvider, err := session.GetSessionProvider()
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if !sessionProvider.Exist(t.SessionID) && (len(t.AuthTokenID) == 0 || !auth.ExistAuthToken(ctx, t.AuthTokenID)) {
			if err := auth.DeleteExternalAuthTokenBySessionID(ctx, t.SessionID); err != nil {
				return err
			}
		}
	}

	return nil
}
