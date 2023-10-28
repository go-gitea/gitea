// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/oauth2"
)

// Ensure the struct implements the interface.
var (
	_ Method = &OAuth2{}
)

// CheckOAuthAccessToken returns uid of user from oauth token
func CheckOAuthAccessToken(ctx context.Context, accessToken string) int64 {
	// JWT tokens require a "."
	if !strings.Contains(accessToken, ".") {
		return 0
	}
	token, err := oauth2.ParseToken(accessToken, oauth2.DefaultSigningKey)
	if err != nil {
		log.Trace("oauth2.ParseToken: %v", err)
		return 0
	}
	var grant *auth_model.OAuth2Grant
	if grant, err = auth_model.GetOAuth2GrantByID(ctx, token.GrantID); err != nil || grant == nil {
		return 0
	}
	if token.Type != oauth2.TypeAccessToken {
		return 0
	}
	if token.ExpiresAt.Before(time.Now()) || token.IssuedAt.After(time.Now()) {
		return 0
	}
	return grant.UserID
}

// OAuth2 implements the Auth interface and authenticates requests
// (API requests only) by looking for an OAuth token in query parameters or the
// "Authorization" header.
type OAuth2 struct{}

// Name represents the name of auth method
func (o *OAuth2) Name() string {
	return "oauth2"
}

// parseToken returns the token from request, and a boolean value
// representing whether the token exists or not
func parseToken(req *http.Request) (string, bool) {
	_ = req.ParseForm()
	// Check token.
	if token := req.Form.Get("token"); token != "" {
		return token, true
	}
	// Check access token.
	if token := req.Form.Get("access_token"); token != "" {
		return token, true
	}
	// check header token
	if auHead := req.Header.Get("Authorization"); auHead != "" {
		auths := strings.Fields(auHead)
		if len(auths) == 2 && (auths[0] == "token" || strings.ToLower(auths[0]) == "bearer") {
			return auths[1], true
		}
	}
	return "", false
}

// userIDFromToken returns the user id corresponding to the OAuth token.
// It will set 'IsApiToken' to true if the token is an API token and
// set 'ApiTokenScope' to the scope of the access token
func (o *OAuth2) userIDFromToken(ctx context.Context, tokenSHA string, store DataStore) int64 {
	// Let's see if token is valid.
	if strings.Contains(tokenSHA, ".") {
		uid := CheckOAuthAccessToken(ctx, tokenSHA)
		if uid != 0 {
			store.GetData()["IsApiToken"] = true
			store.GetData()["ApiTokenScope"] = auth_model.AccessTokenScopeAll // fallback to all
		}
		return uid
	}
	t, err := auth_model.GetAccessTokenBySHA(ctx, tokenSHA)
	if err != nil {
		if auth_model.IsErrAccessTokenNotExist(err) {
			// check task token
			task, err := actions_model.GetRunningTaskByToken(ctx, tokenSHA)
			if err == nil && task != nil {
				log.Trace("Basic Authorization: Valid AccessToken for task[%d]", task.ID)

				store.GetData()["IsActionsToken"] = true
				store.GetData()["ActionsTaskID"] = task.ID

				return user_model.ActionsUserID
			}
		} else if !auth_model.IsErrAccessTokenNotExist(err) && !auth_model.IsErrAccessTokenEmpty(err) {
			log.Error("GetAccessTokenBySHA: %v", err)
		}
		return 0
	}
	t.UpdatedUnix = timeutil.TimeStampNow()
	if err = auth_model.UpdateAccessToken(ctx, t); err != nil {
		log.Error("UpdateAccessToken: %v", err)
	}
	store.GetData()["IsApiToken"] = true
	store.GetData()["ApiTokenScope"] = t.Scope
	return t.UID
}

// Verify extracts the user ID from the OAuth token in the query parameters
// or the "Authorization" header and returns the corresponding user object for that ID.
// If verification is successful returns an existing user object.
// Returns nil if verification fails.
func (o *OAuth2) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	// These paths are not API paths, but we still want to check for tokens because they maybe in the API returned URLs
	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) && !isAuthenticatedTokenRequest(req) &&
		!isGitRawOrAttachPath(req) {
		return nil, nil
	}

	token, ok := parseToken(req)
	if !ok {
		return nil, nil
	}

	id := o.userIDFromToken(req.Context(), token, store)

	if id <= 0 && id != -2 { // -2 means actions, so we need to allow it.
		return nil, user_model.ErrUserNotExist{}
	}
	log.Trace("OAuth2 Authorization: Found token for user[%d]", id)

	user, err := user_model.GetPossibleUserByID(req.Context(), id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
		}
		return nil, err
	}

	log.Trace("OAuth2 Authorization: Logged in user %-v", user)
	return user, nil
}

func isAuthenticatedTokenRequest(req *http.Request) bool {
	switch req.URL.Path {
	case "/login/oauth/userinfo":
		fallthrough
	case "/login/oauth/introspect":
		return true
	}
	return false
}
