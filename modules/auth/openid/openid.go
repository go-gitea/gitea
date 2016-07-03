// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package openid provide functions & structure to authenticate users
// via OpenID
package openid

import (
	"github.com/yohcop/openid-go"
)

// For the demo, we use in-memory infinite storage nonce and discovery
// cache. In your app, do not use this as it will eat up memory and
// never
// free it. Use your own implementation, on a better database system.
// If you have multiple servers for example, you may need to share at
// least
// the nonceStore between them.
var nonceStore = openid.NewSimpleNonceStore()
var discoveryCache = openid.NewSimpleDiscoveryCache()

// LoginViaOpenIDVerification verifies an OpenID URL claim
func LoginViaOpenIDVerification(url string, autoRegister bool) (*User, error) {

	var id, err = openid.Verify(url, discoveryCache, nonceStore)
	if err != nil {
		log.Fatal(1, "Error verifying: %v", err)
	}
	log.Trace("Verified ID: " + id)

	/*
		login := id

		user = &User{
			LowerName:   strings.ToLower(login),
			Name:        login,
			Email:       login,
			Passwd:      nil,
			LoginType:   LoginOpenID,
			LoginSource: sourceID,
			LoginName:   login,
			IsActive:    true,
		}
		return user, CreateUser(user)
	*/
	return nil, nil
}
