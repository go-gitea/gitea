// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	gitea_context "code.gitea.io/gitea/services/context"
	user_service "code.gitea.io/gitea/services/user"
)

type globalVarsStruct struct {
	gitRawOrAttachPathRe *regexp.Regexp
	lfsPathRe            *regexp.Regexp
	archivePathRe        *regexp.Regexp
	feedPathRe           *regexp.Regexp
	feedRefPathRe        *regexp.Regexp
}

var globalVars = sync.OnceValue(func() *globalVarsStruct {
	return &globalVarsStruct{
		gitRawOrAttachPathRe: regexp.MustCompile(`^/[-.\w]+/[-.\w]+/(?:(?:git-(?:(?:upload)|(?:receive))-pack$)|(?:info/refs$)|(?:HEAD$)|(?:objects/)|(?:raw/)|(?:releases/download/)|(?:attachments/))`),
		lfsPathRe:            regexp.MustCompile(`^/[-.\w]+/[-.\w]+/info/lfs/`),
		archivePathRe:        regexp.MustCompile(`^/[-.\w]+/[-.\w]+/archive/`),
		feedPathRe:           regexp.MustCompile(`^/[-.\w]+(/[-.\w]+)?\.(rss|atom)$`), // "/owner.rss" or "/owner/repo.atom"
		feedRefPathRe:        regexp.MustCompile(`^/[-.\w]+/[-.\w]+/(rss|atom)/`),     // "/owner/repo/rss/branch/..."
	}
})

// Init should be called exactly once when the application starts to allow plugins
// to allocate necessary resources
func Init() {
	webauthn.Init()
}

type authPathDetector struct {
	req  *http.Request
	vars *globalVarsStruct
}

func newAuthPathDetector(req *http.Request) *authPathDetector {
	return &authPathDetector{req: req, vars: globalVars()}
}

// isAPIPath returns true if the specified URL is an API path
func (a *authPathDetector) isAPIPath() bool {
	return strings.HasPrefix(a.req.URL.Path, "/api/")
}

// isAttachmentDownload check if request is a file download (GET) with URL to an attachment
func (a *authPathDetector) isAttachmentDownload() bool {
	return strings.HasPrefix(a.req.URL.Path, "/attachments/") && a.req.Method == "GET"
}

func (a *authPathDetector) isFeedRequest(req *http.Request) bool {
	if !setting.Other.EnableFeed {
		return false
	}
	if req.Method != "GET" {
		return false
	}
	return a.vars.feedPathRe.MatchString(req.URL.Path) || a.vars.feedRefPathRe.MatchString(req.URL.Path)
}

// isContainerPath checks if the request targets the container endpoint
func (a *authPathDetector) isContainerPath() bool {
	return strings.HasPrefix(a.req.URL.Path, "/v2/")
}

func (a *authPathDetector) isGitRawOrAttachPath() bool {
	return a.vars.gitRawOrAttachPathRe.MatchString(a.req.URL.Path)
}

func (a *authPathDetector) isGitRawOrAttachOrLFSPath() bool {
	if a.isGitRawOrAttachPath() {
		return true
	}
	if setting.LFS.StartServer {
		return a.vars.lfsPathRe.MatchString(a.req.URL.Path)
	}
	return false
}

func (a *authPathDetector) isArchivePath() bool {
	return a.vars.archivePathRe.MatchString(a.req.URL.Path)
}

func (a *authPathDetector) isAuthenticatedTokenRequest() bool {
	switch a.req.URL.Path {
	case "/login/oauth/userinfo", "/login/oauth/introspect":
		return true
	}
	return false
}

// handleSignIn clears existing session variables and stores new ones for the specified user object
func handleSignIn(resp http.ResponseWriter, req *http.Request, sess SessionStore, user *user_model.User) {
	// We need to regenerate the session...
	newSess, err := session.RegenerateSession(resp, req)
	if err != nil {
		log.Error(fmt.Sprintf("Error regenerating session: %v", err))
	} else {
		sess = newSess
	}

	_ = sess.Delete("openid_verified_uri")
	_ = sess.Delete("openid_signin_remember")
	_ = sess.Delete("openid_determined_email")
	_ = sess.Delete("openid_determined_username")
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("webauthnAssertion")
	_ = sess.Delete("linkAccount")
	err = sess.Set("uid", user.ID)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}
	err = sess.Set("uname", user.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(user.Language) == 0 {
		lc := middleware.Locale(resp, req)
		opts := &user_service.UpdateOptions{
			Language: optional.Some(lc.Language()),
		}
		if err := user_service.UpdateUser(req.Context(), user, opts); err != nil {
			log.Error(fmt.Sprintf("Error updating user language [user: %d, locale: %s]", user.ID, user.Language))
			return
		}
	}

	middleware.SetLocaleCookie(resp, user.Language, 0)

	// force to generate a new CSRF token
	if ctx := gitea_context.GetWebContext(req.Context()); ctx != nil {
		ctx.Csrf.PrepareForSessionUser(ctx)
	}
}
