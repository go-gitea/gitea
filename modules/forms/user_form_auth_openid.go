// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"
	"gitea.com/go-chi/binding"
)

// SignInOpenIDForm form for signing in with OpenID
type SignInOpenIDForm struct {
	Openid   string `binding:"Required;MaxSize(256)"`
	Remember bool
}

// Validate validates the fields
func (f *SignInOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// SignUpOpenIDForm form for signin up with OpenID
type SignUpOpenIDForm struct {
	UserName           string `binding:"Required;AlphaDashDot;MaxSize(40)"`
	Email              string `binding:"Required;Email;MaxSize(254)"`
	GRecaptchaResponse string `form:"g-recaptcha-response"`
	HcaptchaResponse   string `form:"h-captcha-response"`
}

// Validate validates the fields
func (f *SignUpOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// ConnectOpenIDForm form for connecting an existing account to an OpenID URI
type ConnectOpenIDForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
}

// Validate validates the fields
func (f *ConnectOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
