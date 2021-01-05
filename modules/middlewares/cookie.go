// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middlewares

import (
	"net/http"
	"net/url"
	"time"

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

// SetCookie set the cookies
// TODO: Copied from gitea.com/macaron/macaron and should be improved after macaron removed.
func SetCookie(resp http.ResponseWriter, name string, value string, others ...interface{}) {
	cookie := http.Cookie{}
	cookie.Name = name
	cookie.Value = url.QueryEscape(value)

	if len(others) > 0 {
		switch v := others[0].(type) {
		case int:
			cookie.MaxAge = v
		case int64:
			cookie.MaxAge = int(v)
		case int32:
			cookie.MaxAge = int(v)
		case func(*http.Cookie):
			v(&cookie)
		}
	}

	cookie.Path = "/"
	if len(others) > 1 {
		if v, ok := others[1].(string); ok && len(v) > 0 {
			cookie.Path = v
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 2 {
		if v, ok := others[2].(string); ok && len(v) > 0 {
			cookie.Domain = v
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 3 {
		switch v := others[3].(type) {
		case bool:
			cookie.Secure = v
		case func(*http.Cookie):
			v(&cookie)
		default:
			if others[3] != nil {
				cookie.Secure = true
			}
		}
	}

	if len(others) > 4 {
		if v, ok := others[4].(bool); ok && v {
			cookie.HttpOnly = true
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 5 {
		if v, ok := others[5].(time.Time); ok {
			cookie.Expires = v
			cookie.RawExpires = v.Format(time.UnixDate)
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 6 {
		for _, other := range others[6:] {
			if v, ok := other.(func(*http.Cookie)); ok {
				v(&cookie)
			}
		}
	}

	resp.Header().Add("Set-Cookie", cookie.String())
}
