// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import "gitea.dev/modules/web/middleware"

// SignInOpenIDForm form for signing in with OpenID
type SignInOpenIDForm struct {
	middleware.FormDefaultValidator
	Openid   string `binding:"Required;MaxSize(256)"`
	Remember bool
}

// SignUpOpenIDForm form for signin up with OpenID
type SignUpOpenIDForm struct {
	middleware.FormDefaultValidator
	UserName string `binding:"Required;Username;MaxSize(40)"`
	Email    string `binding:"Required;Email;MaxSize(254)"`
}

// ConnectOpenIDForm form for connecting an existing account to an OpenID URI
type ConnectOpenIDForm struct {
	middleware.FormDefaultValidator
	UserName string `binding:"Required;MaxSize(254)"`
	Password string `binding:"Required;MaxSize(255)"`
}
