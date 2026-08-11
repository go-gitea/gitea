// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
)

var _ Method = &DeployToken{}

// DeployToken authenticates a deploy key token given as HTTP basic auth credential.
// Callers only add it to an auth group where a repo scoped credential makes sense,
// currently the git HTTP router, see routers/web/web.go.
type DeployToken struct{}

// Name returns the name of this auth method.
func (d *DeployToken) Name() string {
	return DeployTokenMethodName
}

// Verify resolves the token to its deploy key and returns the owner of the bound
// repository. The key stays on the data store, so that the git HTTP handler can
// limit the request to that repository and to the access mode of the key.
func (d *DeployToken) Verify(req *http.Request, _ http.ResponseWriter, store DataStore, _ SessionStore) (*user_model.User, error) {
	authToken := parseAuthBasic(req).authToken
	if authToken == "" {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	key, err := asymkey_model.VerifyDeployToken(req.Context(), authToken)
	if err != nil {
		if asymkey_model.IsErrDeployKeyNotExist(err) {
			return nil, nil //nolint:nilnil // not a deploy token, let the other methods try
		}
		return nil, err
	}

	repo, err := repo_model.GetRepositoryByID(req.Context(), key.RepoID)
	if err != nil {
		return nil, err
	}
	if err := repo.LoadOwner(req.Context()); err != nil {
		return nil, err
	}

	key.UpdatedUnix = timeutil.TimeStampNow()
	if err := asymkey_model.UpdateDeployKeyCols(req.Context(), key, "updated_unix"); err != nil {
		log.Error("UpdateDeployKeyCols: %v", err)
	}

	log.Trace("Deploy token: valid key for repo[%d]", repo.ID)
	store.GetData()["LoginMethod"] = DeployTokenMethodName
	store.GetData()["DeployKey"] = key
	return repo.Owner, nil
}
