// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

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

// Init should be called exactly once when the application starts to allow plugins
// to allocate necessary resources
func Init() {
	webauthn.Init()
}

// isAttachmentDownload check if request is a file download (GET) with URL to an attachment
func isAttachmentDownload(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/attachments/") && req.Method == "GET"
}

// isFeed checks if the request targets a rss/atom feed
func isFeed(req *http.Request) bool {
	return strings.HasSuffix(req.URL.Path, ".rss") || strings.HasSuffix(req.URL.Path, ".atom")
}

// isContainerPath checks if the request targets the container endpoint
func isContainerPath(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/v2/")
}

var (
	gitRawOrAttachPathRe = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+/(?:(?:git-(?:(?:upload)|(?:receive))-pack$)|(?:info/refs$)|(?:HEAD$)|(?:objects/)|(?:raw/)|(?:releases/download/)|(?:attachments/))`)
	lfsPathRe            = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+/info/lfs/`)
	archivePathRe        = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+/archive/`)
)

func isGitRawOrAttachPath(req *http.Request) bool {
	return gitRawOrAttachPathRe.MatchString(req.URL.Path)
}

func isGitRawOrAttachOrLFSPath(req *http.Request) bool {
	if isGitRawOrAttachPath(req) {
		return true
	}
	if setting.LFS.StartServer {
		return lfsPathRe.MatchString(req.URL.Path)
	}
	return false
}

func isArchivePath(req *http.Request) bool {
	return archivePathRe.MatchString(req.URL.Path)
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
	if ctx := gitea_context.GetWebContext(req); ctx != nil {
		ctx.Csrf.PrepareForSessionUser(ctx)
	}
}
