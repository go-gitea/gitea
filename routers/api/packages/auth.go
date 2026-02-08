// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/packages"
)

var _ auth.Method = &Auth{}

// Auth is for conan and container
type Auth struct {
	AllowGhostUser bool
}

func (a *Auth) Name() string {
	return "packages"
}

// Verify extracts the user from the Bearer token
func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) (*user_model.User, error) {
	packageMeta, err := packages.ParseAuthorizationRequest(req)
	if err != nil {
		log.Trace("ParseAuthorizationToken: %v", err)
		return nil, err
	}

	if packageMeta == nil || packageMeta.UserID == 0 {
		return nil, nil
	}

	var u *user_model.User
	switch packageMeta.UserID {
	case user_model.GhostUserID:
		if !a.AllowGhostUser {
			return nil, nil
		}
		u = user_model.NewGhostUser()
	case user_model.ActionsUserID:
		u = user_model.NewActionsUserWithTaskID(packageMeta.ActionsUserTaskID)
	default:
		u, err = user_model.GetUserByID(req.Context(), packageMeta.UserID)
		if err != nil {
			return nil, err
		}
	}

	if packageMeta.Scope != "" {
		store.GetData()["IsApiToken"] = true
		store.GetData()["ApiTokenScope"] = packageMeta.Scope
	}

	return u, nil
}
