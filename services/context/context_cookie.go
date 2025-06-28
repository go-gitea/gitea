// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

const CookieNameFlash = "gitea_flash"

func removeSessionCookieHeader(w http.ResponseWriter) {
	cookies := w.Header()["Set-Cookie"]
	w.Header().Del("Set-Cookie")
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie, setting.SessionConfig.CookieName+"=") {
			continue
		}
		w.Header().Add("Set-Cookie", cookie)
	}
}

// SetSiteCookie convenience function to set most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) SetSiteCookie(name, value string, maxAge int) {
	middleware.SetSiteCookie(ctx.Resp, name, value, maxAge)
}

// DeleteSiteCookie convenience function to delete most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) DeleteSiteCookie(name string) {
	middleware.SetSiteCookie(ctx.Resp, name, "", -1)
}

// GetSiteCookie returns given cookie value from request header.
func (ctx *Context) GetSiteCookie(name string) string {
	return middleware.GetSiteCookie(ctx.Req, name)
}
