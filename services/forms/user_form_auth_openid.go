// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// SignUpOpenIDForm form for signin up with OpenID
type SignUpOpenIDForm struct {
	UserName string `binding:"Required;Username;MaxSize(40)"`
	Email    string `binding:"Required;Email;MaxSize(254)"`
}

// Validate validates the fields
func (f *SignUpOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// ConnectOpenIDForm form for connecting an existing account to an OpenID URI
type ConnectOpenIDForm struct {
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
}

// Validate validates the fields
func (f *ConnectOpenIDForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
