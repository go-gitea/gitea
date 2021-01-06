// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"net/http"

	"code.gitea.io/gitea/models"
)

// SignedInUser returns the user object of signed user.
// It returns a bool value to indicate whether user uses basic auth or not.
func SignedInUser(req *http.Request, w http.ResponseWriter, ds DataStore, sess SessionStore) (*models.User, bool) {
	if !models.HasEngine {
		return nil, false
	}

	// Try to sign in with each of the enabled plugins
	for _, ssoMethod := range Methods() {
		if !ssoMethod.IsEnabled() {
			continue
		}
		user := ssoMethod.VerifyAuthData(req, w, ds, sess)
		if user != nil {
			_, isBasic := ssoMethod.(*Basic)
			return user, isBasic
		}
	}

	return nil, false
}
