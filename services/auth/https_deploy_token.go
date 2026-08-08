// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/auth/httpauth"
	"gitea.dev/modules/log"
)

// Ensure the struct implements the interface.
var _ Method = &HTTPSDeployToken{}

// HTTPSDeployToken authenticates HTTP Basic-auth credentials whose token half
// matches a row in the https_deploy_key table. It is deliberately *not*
// registered globally: callers add it to an auth group only for request
// contexts where a repo-scoped deploy token makes sense (currently the git
// smart-HTTP router). See routers/web/web.go for the gating flag.
type HTTPSDeployToken struct{}

// Name returns the name of this auth method.
func (h *HTTPSDeployToken) Name() string {
	return HTTPSDeployTokenMethodName
}

// Verify parses the Basic-auth header, resolves the token to an
// HTTPSDeployKey, and returns the bound repository owner. The deploy-key
// metadata is stashed on the data store so downstream permission logic can
// constrain the request to the bound repo and access mode.
func (h *HTTPSDeployToken) Verify(req *http.Request, _ http.ResponseWriter, store DataStore, _ SessionStore) (*user_model.User, error) {
	authToken := extractBasicAuthToken(req)
	if authToken == "" {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	key, err := asymkey_model.VerifyHTTPSDeployToken(req.Context(), authToken)
	if err != nil {
		if asymkey_model.IsErrHTTPSDeployKeyNotExist(err) {
			return nil, nil //nolint:nilnil // not our token — fall through to regular basic auth
		}
		return nil, err
	}

	repo, err := repo_model.GetRepositoryByID(req.Context(), key.RepoID)
	if err != nil {
		log.Error("HTTPSDeployToken: GetRepositoryByID(%d): %v", key.RepoID, err)
		return nil, err
	}
	if err := repo.LoadOwner(req.Context()); err != nil {
		log.Error("HTTPSDeployToken: LoadOwner for repo %d: %v", repo.ID, err)
		return nil, err
	}

	log.Trace("HTTPSDeployToken: valid HTTPS deploy key for repo[%d]", repo.ID)
	store.GetData()["LoginMethod"] = HTTPSDeployTokenMethodName
	store.GetData()["IsDeployToken"] = true
	store.GetData()["DeployTokenID"] = key.ID
	store.GetData()["DeployTokenRepoID"] = key.RepoID
	store.GetData()["DeployTokenMode"] = key.Mode
	return repo.Owner, nil
}

// extractBasicAuthToken pulls the credential string out of an HTTP Basic
// Authorization header. It returns the password half when present (the
// conventional token-in-password pattern used by git credential helpers) and
// otherwise the username half. Returns "" if no Basic header is present.
func extractBasicAuthToken(req *http.Request) string {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parsed, ok := httpauth.ParseAuthorizationHeader(authHeader)
	if !ok || parsed.BasicAuth == nil {
		return ""
	}
	if parsed.BasicAuth.Password != "" && parsed.BasicAuth.Password != "x-oauth-basic" {
		return parsed.BasicAuth.Password
	}
	return parsed.BasicAuth.Username
}
