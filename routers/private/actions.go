// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/util"
)

// GenerateActionsRunnerToken generates a new runner token for a given scope
func GenerateActionsRunnerToken(ctx *context.PrivateContext) {
	var genRequest private.GenerateTokenRequest
	rd := ctx.Req.Body
	defer rd.Close()

	if err := json.NewDecoder(rd).Decode(&genRequest); err != nil {
		log.Error("%v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	owner, repo, err := parseScope(ctx, genRequest.Scope)
	if err != nil {
		log.Error("%v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
	}

	token, err := actions_model.GetUnactivatedRunnerToken(ctx, owner, repo)
	if errors.Is(err, util.ErrNotExist) {
		token, err = actions_model.NewRunnerToken(ctx, owner, repo)
		if err != nil {
			err := fmt.Sprintf("error while creating runner token: %v", err)
			log.Error("%v", err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err,
			})
			return
		}
	} else if err != nil {
		err := fmt.Sprintf("could not get unactivated runner token: %v", err)
		log.Error("%v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err,
		})
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

	r, err := repo_model.GetRepositoryByName(u.ID, repoName)
	if err != nil {
		return ownerID, repoID, err
	}
	repoID = r.ID
	return ownerID, repoID, nil
}
