// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	deploykey_model "gitea.dev/models/deploykey"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
)

var _ Method = &DeployToken{}

// DeployToken authenticates a deploy key token given as HTTP basic auth credential.
// Only add it to an auth group where a repo scoped credential makes sense.
type DeployToken struct{}

func (d *DeployToken) Name() string {
	return DeployTokenMethodName
}

// Verify returns a user that stands for the deploy key alone. Its permissions come from the key,
// see access_model.getDeployKeyRepoPermission, so the request can never reach another repository
// or exceed the access mode of the key.
func (d *DeployToken) Verify(req *http.Request, _ http.ResponseWriter, store DataStore, _ SessionStore) (*user_model.User, error) {
	authToken := parseAuthBasic(req).authToken
	if authToken == "" {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	key, err := deploykey_model.VerifyDeployToken(req.Context(), authToken)
	if err != nil {
		if deploykey_model.IsErrDeployKeyNotExist(err) {
			return nil, nil //nolint:nilnil // not a deploy token, let the other methods try
		}
		return nil, err
	}

	if err := deploykey_model.UpdateDeployKeyUpdated(req.Context(), key.ID); err != nil {
		log.Error("UpdateDeployKeyUpdated: %v", err)
	}

	store.GetData()["LoginMethod"] = DeployTokenMethodName
	return user_model.NewDeployKeyUserWithKeyID(key.ID), nil
}
