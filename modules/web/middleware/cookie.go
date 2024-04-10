// Copyright 2020 The Macaron Authors
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/url"
	"strings"

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
	cookie := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     "", // Filled in below.
		Domain:   setting.SessionConfig.Domain,
		Secure:   setting.SessionConfig.Secure,
		HttpOnly: true,
		SameSite: setting.SessionConfig.SameSite,
	}
	if maxAge < 0 {
		// There was a bug in "setting.SessionConfig.CookiePath" code, the old default value of it was empty "".
		// So we have to delete the cookie on path="" again, because some old code leaves cookies on path="".
		// The code was updated, but it behaves differently depending on the
		// value of AppSubURL.  When AppSubURL is non-empty, the cookie with a
		// trailing slash must be deleted.
		withoutTrailingSlash := strings.TrimSuffix(setting.SessionConfig.CookiePath, "/")
		withTrailingSlash := withoutTrailingSlash + "/"
		for _, path := range []string{withoutTrailingSlash, withTrailingSlash} {
			cookie.Path = path
			resp.Header().Add("Set-Cookie", cookie.String())
		}
	} else {
		cookie.Path = setting.SessionConfig.CookiePath
		resp.Header().Add("Set-Cookie", cookie.String())
	}
}
