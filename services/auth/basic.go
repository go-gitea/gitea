// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/httpauth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// Ensure the struct implements the interface.
var (
	_ Method = &Basic{}
)

// BasicMethodName is the constant name of the basic authentication method
const (
	BasicMethodName       = "basic"
	AccessTokenMethodName = "access_token"
	OAuth2TokenMethodName = "oauth2_token"
	ActionTokenMethodName = "action_token"
)

// Basic implements the Auth interface and authenticates requests (API requests
// only) by looking for Basic authentication data or "x-oauth-basic" token in the "Authorization"
// header.
type Basic struct{}

// Name represents the name of auth method
func (b *Basic) Name() string {
	return BasicMethodName
}

// Verify extracts and validates Basic data (username and password/token) from the
// "Authorization" header of the request and returns the corresponding user object for that
// name/token on successful validation.
// Returns nil if header is empty or validation fails.
func (b *Basic) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	// Basic authentication should only fire on API, Feed, Download, Archives or on Git or LFSPaths
	// Not all feed (rss/atom) clients feature the ability to add cookies or headers, so we need to allow basic auth for feeds
	detector := newAuthPathDetector(req)
	if !detector.isAPIPath() && !detector.isFeedRequest(req) && !detector.isContainerPath() && !detector.isAttachmentDownload() && !detector.isArchivePath() && !detector.isGitRawOrAttachOrLFSPath() {
		return nil, nil
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}
	parsed, ok := httpauth.ParseAuthorizationHeader(authHeader)
	if !ok || parsed.BasicAuth == nil {
		return nil, nil
	}
	uname, passwd := parsed.BasicAuth.Username, parsed.BasicAuth.Password

	// Check if username or password is a token
	isUsernameToken := len(passwd) == 0 || passwd == "x-oauth-basic"
	// Assume username is token
	authToken := uname
	if !isUsernameToken {
		log.Trace("Basic Authorization: Attempting login for: %s", uname)
		// Assume password is token
		authToken = passwd
	} else {
		log.Trace("Basic Authorization: Attempting login with username as token")
	}

	// get oauth2 token's user's ID
	_, uid := GetOAuthAccessTokenScopeAndUserID(req.Context(), authToken)
	if uid != 0 {
		log.Trace("Basic Authorization: Valid OAuthAccessToken for user[%d]", uid)

		u, err := user_model.GetUserByID(req.Context(), uid)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil, err
		}

		store.GetData()["LoginMethod"] = OAuth2TokenMethodName
		store.GetData()["IsApiToken"] = true
		return u, nil
	}

	// check personal access token
	token, err := auth_model.GetAccessTokenBySHA(req.Context(), authToken)
	if err == nil {
		log.Trace("Basic Authorization: Valid AccessToken for user[%d]", uid)
		u, err := user_model.GetUserByID(req.Context(), token.UID)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil, err
		}

		token.UpdatedUnix = timeutil.TimeStampNow()
		if err = auth_model.UpdateAccessToken(req.Context(), token); err != nil {
			log.Error("UpdateAccessToken:  %v", err)
		}

		store.GetData()["LoginMethod"] = AccessTokenMethodName
		store.GetData()["IsApiToken"] = true
		store.GetData()["ApiTokenScope"] = token.Scope
		return u, nil
	} else if !auth_model.IsErrAccessTokenNotExist(err) && !auth_model.IsErrAccessTokenEmpty(err) {
		log.Error("GetAccessTokenBySha: %v", err)
	}

	// check task token
	task, err := actions_model.GetRunningTaskByToken(req.Context(), authToken)
	if err == nil && task != nil {
		log.Trace("Basic Authorization: Valid AccessToken for task[%d]", task.ID)

		store.GetData()["LoginMethod"] = ActionTokenMethodName
		store.GetData()["IsActionsToken"] = true
		store.GetData()["ActionsTaskID"] = task.ID

		return user_model.NewActionsUser(), nil
	}

	if !setting.Service.EnableBasicAuth {
		return nil, nil
	}

	log.Trace("Basic Authorization: Attempting SignIn for %s", uname)
	u, source, err := UserSignIn(req.Context(), uname, passwd)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("UserSignIn: %v", err)
		}
		return nil, err
	}

	if !source.TwoFactorShouldSkip() {
		// Check if the user has WebAuthn registration
		hasWebAuthn, err := auth_model.HasWebAuthnRegistrationsByUID(req.Context(), u.ID)
		if err != nil {
			return nil, err
		}
		if hasWebAuthn {
			return nil, errors.New("basic authorization is not allowed while WebAuthn enrolled")
		}

		if err := validateTOTP(req, u); err != nil {
			return nil, err
		}
	}

	store.GetData()["LoginMethod"] = BasicMethodName
	log.Trace("Basic Authorization: Logged in user %-v", u)

	return u, nil
}

func validateTOTP(req *http.Request, u *user_model.User) error {
	twofa, err := auth_model.GetTwoFactorByUID(req.Context(), u.ID)
	if err != nil {
		if auth_model.IsErrTwoFactorNotEnrolled(err) {
			// No 2FA enrollment for this user
			return nil
		}
		return err
	}
	if ok, err := twofa.ValidateTOTP(req.Header.Get("X-Gitea-OTP")); err != nil {
		return err
	} else if !ok {
		return util.NewInvalidArgumentErrorf("invalid provided OTP")
	}
	return nil
}

func GetAccessScope(store DataStore) auth_model.AccessTokenScope {
	if v, ok := store.GetData()["ApiTokenScope"]; ok {
		return v.(auth_model.AccessTokenScope)
	}
	switch store.GetData()["LoginMethod"] {
	case OAuth2TokenMethodName:
		fallthrough
	case BasicMethodName, AccessTokenMethodName:
		return auth_model.AccessTokenScopeAll
	case ActionTokenMethodName:
		fallthrough
	default:
		return ""
	}
}
