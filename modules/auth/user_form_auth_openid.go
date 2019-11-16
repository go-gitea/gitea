// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"gitea.com/macaron/binding"
	"gitea.com/macaron/macaron"
)

// SignInOpenIDForm form for signing in with OpenID
type SignInOpenIDForm struct {
	Openid   string `binding:"Required;MaxSize(256)"`
	Remember bool
}

// Validate valideates the fields
func (f *SignInOpenIDForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// SignUpOpenIDForm form for signin up with OpenID
type SignUpOpenIDForm struct {
	UserName           string `binding:"Required;AlphaDashDot;MaxSize(40)"`
	Email              string `binding:"Required;Email;MaxSize(254)"`
	GRecaptchaResponse string `form:"g-recaptcha-response"`
}

// Validate valideates the fields
func (f *SignUpOpenIDForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

// ConnectOpenIDForm form for connecting an existing account to an OpenID URI
type ConnectOpenIDForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
}

// Validate valideates the fields
func (f *ConnectOpenIDForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}
