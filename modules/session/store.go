// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/session"
)

// Store represents a session store
type Store interface {
	Get(any) any
	Set(any, any) error
	Delete(any) error
}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	// Ensure that a cookie with a trailing slash does not take precedence over
	// the cookie written by the middleware.
	middleware.DeleteLegacySiteCookie(resp, setting.SessionConfig.CookieName)

	s, err := session.RegenerateSession(resp, req)
	return s, err
}
