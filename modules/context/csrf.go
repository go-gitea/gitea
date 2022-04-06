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

// a middleware that generates and validates CSRF tokens.

package context

import (
	"encoding/base32"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
)

// CSRFProtector represents a CSRF protector and is used to get the current token and validate the token.
type CSRFProtector interface {
	// GetHeaderName returns HTTP header to search for token.
	GetHeaderName() string
	// GetFormName returns form value to search for token.
	GetFormName() string
	// GetToken returns the token.
	GetToken() string
	// Validate validates the token in http context.
	Validate(ctx *Context)
}

type csrfProtector struct {
	// Header name value for setting and getting CSRF token.
	Header string
	// Form name value for setting and getting CSRF token.
	Form string
	// Cookie name value for setting and getting CSRF token.
	Cookie string
	// Cookie domain
	CookieDomain string
	// Cookie path
	CookiePath string
	// Cookie HttpOnly flag value used for the CSRF token.
	CookieHTTPOnly bool
	// Token generated to pass via header, cookie, or hidden form value.
	Token string
	// This value must be unique per user.
	ID string
	// Secret used along with the unique id above to generate the Token.
	Secret string
}

// GetHeaderName returns the name of the HTTP header for CSRF token.
func (c *csrfProtector) GetHeaderName() string {
	return c.Header
}

// GetFormName returns the name of the form value for CSRF token.
func (c *csrfProtector) GetFormName() string {
	return c.Form
}

// GetToken returns the current token. This is typically used
// to populate a hidden form in an HTML template.
func (c *csrfProtector) GetToken() string {
	return c.Token
}

// CsrfOptions maintains options to manage behavior of Generate.
type CsrfOptions struct {
	// The global secret value used to generate Tokens.
	Secret string
	// HTTP header used to set and get token.
	Header string
	// Form value used to set and get token.
	Form string
	// Cookie value used to set and get token.
	Cookie string
	// Cookie domain.
	CookieDomain string
	// Cookie path.
	CookiePath     string
	CookieHTTPOnly bool
	// SameSite set the cookie SameSite type
	SameSite http.SameSite
	// Key used for getting the unique ID per user.
	SessionKey string
	// oldSessionKey saves old value corresponding to SessionKey.
	oldSessionKey string
	// If true, send token via X-Csrf-Token header.
	SetHeader bool
	// If true, send token via _csrf cookie.
	SetCookie bool
	// Set the Secure flag to true on the cookie.
	Secure bool
	// Disallow Origin appear in request header.
	Origin bool
	// Cookie lifetime. Default is 0
	CookieLifeTime int
}

func prepareDefaultCsrfOptions(opt CsrfOptions) CsrfOptions {
	if opt.Secret == "" {
		randBytes, err := util.CryptoRandomBytes(8)
		if err != nil {
			// this panic can be handled by the recover() in http handlers
			panic(fmt.Errorf("failed to generate random bytes: %w", err))
		}
		opt.Secret = base32.StdEncoding.EncodeToString(randBytes)
	}
	if opt.Header == "" {
		opt.Header = "X-Csrf-Token"
	}
	if opt.Form == "" {
		opt.Form = "_csrf"
	}
	if opt.Cookie == "" {
		opt.Cookie = "_csrf"
	}
	if opt.CookiePath == "" {
		opt.CookiePath = "/"
	}
	if opt.SessionKey == "" {
		opt.SessionKey = "uid"
	}
	opt.oldSessionKey = "_old_" + opt.SessionKey
	return opt
}

// NewCSRFProtector returns a CSRFProtector to be used for every request.
// Additionally, depending on options set, generated tokens will be sent via Header and/or Cookie.
func NewCSRFProtector(opt CsrfOptions, ctx *Context) CSRFProtector {
	opt = prepareDefaultCsrfOptions(opt)
	x := &csrfProtector{
		Secret:         opt.Secret,
		Header:         opt.Header,
		Form:           opt.Form,
		Cookie:         opt.Cookie,
		CookieDomain:   opt.CookieDomain,
		CookiePath:     opt.CookiePath,
		CookieHTTPOnly: opt.CookieHTTPOnly,
	}

	if opt.Origin && len(ctx.Req.Header.Get("Origin")) > 0 {
		return x
	}

	x.ID = "0"
	uidAny := ctx.Session.Get(opt.SessionKey)
	if uidAny != nil {
		switch uidVal := uidAny.(type) {
		case string:
			x.ID = uidVal
		case int64:
			x.ID = strconv.FormatInt(uidVal, 10)
		default:
			log.Error("invalid uid type in session: %T", uidAny)
		}
	}

	oldUID := ctx.Session.Get(opt.oldSessionKey)
	uidChanged := oldUID == nil || oldUID.(string) != x.ID
	cookieToken := ctx.GetCookie(opt.Cookie)

	needsNew := true
	if uidChanged {
		_ = ctx.Session.Set(opt.oldSessionKey, x.ID)
	} else if cookieToken != "" {
		// If cookie token presents, re-use existing unexpired token, else generate a new one.
		if issueTime, ok := ParseCsrfToken(x.Token); ok {
			dur := time.Since(issueTime)
			if dur >= -CsrfTokenRegenerationDuration && dur <= CsrfTokenRegenerationDuration {
				x.Token = cookieToken
				needsNew = false
			}
		}
	}

	if needsNew {
		// FIXME: actionId.
		x.Token = GenerateCsrfToken(x.Secret, x.ID, "POST", time.Now())
		if opt.SetCookie {
			var expires interface{}
			if opt.CookieLifeTime == 0 {
				expires = time.Now().AddDate(0, 0, 1)
			}
			middleware.SetCookie(ctx.Resp, opt.Cookie, x.Token,
				opt.CookieLifeTime,
				opt.CookiePath,
				opt.CookieDomain,
				opt.Secure,
				opt.CookieHTTPOnly,
				expires,
				middleware.SameSite(opt.SameSite),
			)
		}
	}

	if opt.SetHeader {
		ctx.Resp.Header().Add(opt.Header, x.Token)
	}
	return x
}

func (c *csrfProtector) validateToken(ctx *Context, token string) bool {
	if !ValidCsrfToken(token, c.Secret, c.ID, "POST", time.Now()) {
		middleware.DeleteCSRFCookie(ctx.Resp)
		if middleware.IsAPIPath(ctx.Req) {
			http.Error(ctx.Resp, "Invalid CSRF token.", http.StatusBadRequest)
		} else {
			ctx.Flash.Error(ctx.Tr("error.invalid_csrf"))
			ctx.Redirect(setting.AppSubURL + "/")
		}
		return false
	}
	return true
}

// Validate should be used as a per route middleware. It attempts to get a token from an "X-Csrf-Token"
// HTTP header and then a "_csrf" form value. If one of these is found, the token will be validated
// using ValidToken. If this validation fails, custom Error is sent in the reply.
// If neither a header nor form value is found, http.StatusBadRequest is sent.
func (c *csrfProtector) Validate(ctx *Context) {
	if token := ctx.Req.Header.Get(c.GetHeaderName()); token != "" {
		if c.validateToken(ctx, token) {
			return
		}
	}
	if token := ctx.Req.FormValue(c.GetFormName()); token != "" {
		if c.validateToken(ctx, token) {
			return
		}
	}
	c.validateToken(ctx, "") // no csrf token, use an empty token to respond error
}
