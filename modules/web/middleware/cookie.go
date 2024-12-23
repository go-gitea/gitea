// Copyright 2020 The Macaron Authors
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
)

// SetRedirectToCookie convenience function to set the RedirectTo cookie consistently
func SetRedirectToCookie(resp http.ResponseWriter, value string) {
	SetSiteCookie(resp, "redirect_to", value, 0)
}

// DeleteRedirectToCookie convenience function to delete most cookies consistently
func DeleteRedirectToCookie(resp http.ResponseWriter) {
	SetSiteCookie(resp, "redirect_to", "", -1)
}

// GetSiteCookie returns given cookie value from request header.
func GetSiteCookie(req *http.Request, name string) string {
	cookie, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val
}

// SetSiteCookie returns given cookie value from request header.
func SetSiteCookie(resp http.ResponseWriter, name, value string, maxAge int) {
	// Previous versions would use a cookie path with a trailing /.
	// These are more specific than cookies without a trailing /, so
	// we need to delete these if they exist.
	deleteLegacySiteCookie(resp, name)
	cookie := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     setting.SessionConfig.CookiePath,
		Domain:   setting.SessionConfig.Domain,
		Secure:   setting.SessionConfig.Secure,
		HttpOnly: true,
		SameSite: setting.SessionConfig.SameSite,
	}
	resp.Header().Add("Set-Cookie", cookie.String())
}

// deleteLegacySiteCookie deletes the cookie with the given name at the cookie
// path with a trailing /, which would unintentionally override the cookie.
func deleteLegacySiteCookie(resp http.ResponseWriter, name string) {
	if setting.SessionConfig.CookiePath == "" || strings.HasSuffix(setting.SessionConfig.CookiePath, "/") {
		// If the cookie path ends with /, no legacy cookies will take
		// precedence, so do nothing.  The exception is that cookies with no
		// path could override other cookies, but it's complicated and we don't
		// currently handle that.
		return
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     setting.SessionConfig.CookiePath + "/",
		Domain:   setting.SessionConfig.Domain,
		Secure:   setting.SessionConfig.Secure,
		HttpOnly: true,
		SameSite: setting.SessionConfig.SameSite,
	}
	resp.Header().Add("Set-Cookie", cookie.String())
}

func init() {
	session.BeforeRegenerateSession = append(session.BeforeRegenerateSession, func(resp http.ResponseWriter, _ *http.Request) {
		// Ensure that a cookie with a trailing slash does not take precedence over
		// the cookie written by the middleware.
		deleteLegacySiteCookie(resp, setting.SessionConfig.CookieName)
	})
}
