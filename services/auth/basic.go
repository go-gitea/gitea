// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web/middleware"
)

// Ensure the struct implements the interface.
var (
	_ Method = &Basic{}
	_ Named  = &Basic{}
)

// BasicMethodName is the constant name of the basic authentication method
const BasicMethodName = "basic"

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
func (b *Basic) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *user_model.User {
	// Basic authentication should only fire on API, Download or on Git or LFSPaths
	if !middleware.IsAPIPath(req) && !isContainerPath(req) && !isAttachmentDownload(req) && !isGitRawReleaseOrLFSPath(req) {
		return nil
	}

	baHead := req.Header.Get("Authorization")
	if len(baHead) == 0 {
		return nil
	}

	auths := strings.SplitN(baHead, " ", 2)
	if len(auths) != 2 || (strings.ToLower(auths[0]) != "basic") {
		return nil
	}

	uname, passwd, _ := base.BasicAuthDecode(auths[1])

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

	// check oauth2 token
	uid := CheckOAuthAccessToken(authToken)
	if uid != 0 {
		log.Trace("Basic Authorization: Valid OAuthAccessToken for user[%d]", uid)

		u, err := user_model.GetUserByID(uid)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil
		}

		store.GetData()["IsApiToken"] = true
		return u
	}

	// check personal access token
	token, err := auth_model.GetAccessTokenBySHA(authToken)
	if err == nil {
		log.Trace("Basic Authorization: Valid AccessToken for user[%d]", uid)
		u, err := user_model.GetUserByID(token.UID)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil
		}

		token.UpdatedUnix = timeutil.TimeStampNow()
		if err = auth_model.UpdateAccessToken(token); err != nil {
			log.Error("UpdateAccessToken:  %v", err)
		}

		store.GetData()["IsApiToken"] = true
		return u
	} else if !auth_model.IsErrAccessTokenNotExist(err) && !auth_model.IsErrAccessTokenEmpty(err) {
		log.Error("GetAccessTokenBySha: %v", err)
	}

	// check task token
	task, err := bots_model.GetTaskByToken(db.DefaultContext, authToken)
	if err == nil && task != nil && task.Status.IsRunning() {
		log.Trace("Basic Authorization: Valid AccessToken for task[%d]", task.ID)

		store.GetData()["IsBotToken"] = true
		store.GetData()["BotTaskID"] = task.ID

		return user_model.NewBotUser()
	} else {
		log.Error("GetRunnerByToken: %v %v", task, err)
	}

	if !setting.Service.EnableBasicAuth {
		return nil
	}

	log.Trace("Basic Authorization: Attempting SignIn for %s", uname)
	u, source, err := UserSignIn(uname, passwd)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("UserSignIn: %v", err)
		}
		return nil
	}

	if skipper, ok := source.Cfg.(LocalTwoFASkipper); ok && skipper.IsSkipLocalTwoFA() {
		store.GetData()["SkipLocalTwoFA"] = true
	}

	log.Trace("Basic Authorization: Logged in user %-v", u)

	return u
}
