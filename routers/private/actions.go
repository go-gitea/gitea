// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"
	"strings"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/private"
	"gitea.dev/services/context"
)

// GenerateActionsRunnerToken generates a new runner token for a given scope
func GenerateActionsRunnerToken(ctx *context.PrivateContext) {
	var genRequest private.GenerateTokenRequest
	rd := ctx.Req.Body
	defer rd.Close()

	if err := json.NewDecoder(rd).Decode(&genRequest); err != nil {
		ctx.PrivateInternalErrorf("JSON Decode failed: %v", err)
		return
	}

	owner, repo, err := parseScope(ctx, genRequest.Scope)
	if err != nil {
		ctx.PrivateInternalErrorf("parseScope failed: %v", err)
		return
	}

	token, err := actions_model.GetOrCreateActiveRunnerToken(ctx, owner, repo)
	if err != nil {
		ctx.PrivateInternalErrorf("GetOrCreateActiveRunnerToken failed: %v", err)
		return
	}

	ctx.PlainText(http.StatusOK, token.Token)
}

func parseScope(ctx *context.PrivateContext, scope string) (ownerID, repoID int64, err error) {
	ownerID = 0
	repoID = 0
	if scope == "" {
		return ownerID, repoID, nil
	}

	ownerName, repoName, found := strings.Cut(scope, "/")

	u, err := user_model.GetUserByName(ctx, ownerName)
	if err != nil {
		return ownerID, repoID, err
	}
	ownerID = u.ID

	if !found {
		return ownerID, repoID, nil
	}

	r, err := repo_model.GetRepositoryByName(ctx, u.ID, repoName)
	if err != nil {
		return ownerID, repoID, err
	}
	repoID = r.ID
	return ownerID, repoID, nil
}
