// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middlewares

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"
)

// NewCookie creates a cookie
func NewCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Path:     setting.SessionConfig.CookiePath,
		Domain:   setting.SessionConfig.Domain,
		MaxAge:   maxAge,
		Secure:   setting.SessionConfig.Secure,
	}
}
