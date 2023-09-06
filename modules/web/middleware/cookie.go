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
func SetRedirectToCookie(resp http.ResponseWriter, req *http.Request, value string) {
	SetSiteCookie(resp, req, "redirect_to", value, 0)
}

// DeleteRedirectToCookie convenience function to delete most cookies consistently
func DeleteRedirectToCookie(resp http.ResponseWriter, req *http.Request) {
	SetSiteCookie(resp, req, "redirect_to", "", -1)
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

// GetCookieSecure returns whether the "Secure" attribute on a cookie should be set
func GetCookieSecure(req *http.Request) bool {
	forwardedProto := req.Header.Get("x-forwarded-proto")
	if forwardedProto != "" {
		return forwardedProto == "https"
	} else {
		return req.TLS != nil
	}
}

// SetSiteCookie returns given cookie value from request header.
func SetSiteCookie(resp http.ResponseWriter, req *http.Request, name, value string, maxAge int) {

	cookie := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     setting.SessionConfig.CookiePath,
		Domain:   setting.SessionConfig.Domain,
		Secure:   GetCookieSecure(req),
		HttpOnly: true,
		SameSite: setting.SessionConfig.SameSite,
	}
	resp.Header().Add("Set-Cookie", cookie.String())
	if maxAge < 0 {
		// There was a bug in "setting.SessionConfig.CookiePath" code, the old default value of it was empty "".
		// So we have to delete the cookie on path="" again, because some old code leaves cookies on path="".
		cookie.Path = strings.TrimSuffix(setting.SessionConfig.CookiePath, "/")
		resp.Header().Add("Set-Cookie", cookie.String())
	}
}
