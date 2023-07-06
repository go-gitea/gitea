// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
)

// Shortcut is a group of verification methods that is like Group.
// It tries each method in order and returns the first non-nil user,
// but it never returns error even if some method returns error.
// It is useful for some methods that share the same protocol, shortcut can check them first.
// For example, OAuth2 and conan.Auth both read token from "Authorization: Bearer <token>" header,
// If OAuth2 returns error, it is possible that the token is for conan.Auth but it has no chance to check.
// And Shortcut solves this problem by:
//
//	NewGroup(
//	    Shortcut{&OAuth2, &conan.Auth},
//	    &OAuth2,
//	    &auth.Basic{},
//	    &nuget.Auth{},
//	    &conan.Auth{},
//	    &chef.Auth{},
//	)
//
// Since Shortcut will set "AuthedMethod" in DataStore if any method returns non-nil user,
// so it is unnecessary to implement Named interface for it, the "name" of Shortcut should never be stored as "AuthedMethod".
type Shortcut []Method

func (s Shortcut) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	for _, method := range s {
		user, err := method.Verify(req, w, store, sess)
		if err != nil {
			// Don't return error, just try next method
			continue
		}

		if user != nil {
			if store.GetData()["AuthedMethod"] == nil {
				if named, ok := method.(Named); ok {
					store.GetData()["AuthedMethod"] = named.Name()
				}
			}
			return user, nil
		}
	}

	return nil, nil
}
