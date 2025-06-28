// Copyright 2013 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2021 The Gitea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.
// SPDX-License-Identifier: Apache-2.0

// a middleware that generates and validates CSRF tokens.

package context

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const (
	CsrfHeaderName = "X-Csrf-Token"
	CsrfFormName   = "_csrf"
)

// CSRFProtector represents a CSRF protector and is used to get the current token and validate the token.
type CSRFProtector interface {
	// PrepareForSessionUser prepares the csrf protector for the current session user.
	PrepareForSessionUser(ctx *Context)
	// Validate validates the csrf token in http context.
	Validate(ctx *Context)
	// DeleteCookie deletes the csrf cookie
	DeleteCookie(ctx *Context)
}

type csrfProtector struct {
	opt CsrfOptions
	// id must be unique per user.
	id string
	// token is the valid one which wil be used by end user and passed via header, cookie, or hidden form value.
	token string
}

// CsrfOptions maintains options to manage behavior of Generate.
type CsrfOptions struct {
	// The global secret value used to generate Tokens.
	Secret string
	// Cookie value used to set and get token.
	Cookie string
	// Cookie domain.
	CookieDomain string
	// Cookie path.
	CookiePath     string
	CookieHTTPOnly bool
	// SameSite set the cookie SameSite type
	SameSite http.SameSite
	// Set the Secure flag to true on the cookie.
	Secure bool
	// sessionKey is the key used for getting the unique ID per user.
	sessionKey string
	// oldSessionKey saves old value corresponding to sessionKey.
	oldSessionKey string
}

func newCsrfCookie(opt *CsrfOptions, value string) *http.Cookie {
	return &http.Cookie{
		Name:     opt.Cookie,
		Value:    value,
		Path:     opt.CookiePath,
		Domain:   opt.CookieDomain,
		MaxAge:   int(CsrfTokenTimeout.Seconds()),
		Secure:   opt.Secure,
		HttpOnly: opt.CookieHTTPOnly,
		SameSite: opt.SameSite,
	}
}

func NewCSRFProtector(opt CsrfOptions) CSRFProtector {
	if opt.Secret == "" {
		panic("CSRF secret is empty but it must be set") // it shouldn't happen because it is always set in code
	}
	opt.Cookie = util.IfZero(opt.Cookie, "_csrf")
	opt.CookiePath = util.IfZero(opt.CookiePath, "/")
	opt.sessionKey = "uid"
	opt.oldSessionKey = "_old_" + opt.sessionKey
	return &csrfProtector{opt: opt}
}

func (c *csrfProtector) PrepareForSessionUser(ctx *Context) {
	c.id = "0"
	if uidAny := ctx.Session.Get(c.opt.sessionKey); uidAny != nil {
		switch uidVal := uidAny.(type) {
		case string:
			c.id = uidVal
		case int64:
			c.id = strconv.FormatInt(uidVal, 10)
		default:
			log.Error("invalid uid type in session: %T", uidAny)
		}
	}

	oldUID := ctx.Session.Get(c.opt.oldSessionKey)
	uidChanged := oldUID == nil || oldUID.(string) != c.id
	cookieToken := ctx.GetSiteCookie(c.opt.Cookie)

	needsNew := true
	if uidChanged {
		_ = ctx.Session.Set(c.opt.oldSessionKey, c.id)
	} else if cookieToken != "" {
		// If cookie token presents, re-use existing unexpired token, else generate a new one.
		if issueTime, ok := ParseCsrfToken(cookieToken); ok {
			dur := time.Since(issueTime) // issueTime is not a monotonic-clock, the server time may change a lot to an early time.
			if dur >= -CsrfTokenRegenerationInterval && dur <= CsrfTokenRegenerationInterval {
				c.token = cookieToken
				needsNew = false
			}
		}
	}

	if needsNew {
		c.token = GenerateCsrfToken(c.opt.Secret, c.id, "POST", time.Now())
		ctx.Resp.Header().Add("Set-Cookie", newCsrfCookie(&c.opt, c.token).String())
	}

	ctx.Data["CsrfToken"] = c.token
	ctx.Data["CsrfTokenHtml"] = template.HTML(`<input type="hidden" name="_csrf" value="` + template.HTMLEscapeString(c.token) + `">`)
}

func (c *csrfProtector) validateToken(ctx *Context, token string) {
	if !ValidCsrfToken(token, c.opt.Secret, c.id, "POST", time.Now()) {
		c.DeleteCookie(ctx)
		// currently, there should be no access to the APIPath with CSRF token. because templates shouldn't use the `/api/` endpoints.
		// FIXME: distinguish what the response is for: HTML (web page) or JSON (fetch)
		http.Error(ctx.Resp, "Invalid CSRF token.", http.StatusBadRequest)
	}
}

// Validate should be used as a per route middleware. It attempts to get a token from an "X-Csrf-Token"
// HTTP header and then a "_csrf" form value. If one of these is found, the token will be validated.
// If this validation fails, http.StatusBadRequest is sent.
func (c *csrfProtector) Validate(ctx *Context) {
	if token := ctx.Req.Header.Get(CsrfHeaderName); token != "" {
		c.validateToken(ctx, token)
		return
	}
	if token := ctx.Req.FormValue(CsrfFormName); token != "" {
		c.validateToken(ctx, token)
		return
	}
	c.validateToken(ctx, "") // no csrf token, use an empty token to respond error
}

func (c *csrfProtector) DeleteCookie(ctx *Context) {
	cookie := newCsrfCookie(&c.opt, "")
	cookie.MaxAge = -1
	ctx.Resp.Header().Add("Set-Cookie", cookie.String())
}
