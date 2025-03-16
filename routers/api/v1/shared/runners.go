// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// RegistrationToken is response related to registration token
// swagger:response RegistrationToken
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	token, err := actions_model.GetLatestRunnerToken(ctx, ownerID, repoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, ownerID, repoID)
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}

func ListRunners(ctx *context.APIContext, ownerID, repoID int64) {
	runners, total, err := db.FindAndCount[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	res := new(api.ActionRunnersResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionRunner, len(runners))
	for i, runner := range runners {
		res.Entries[i] = convert.ToActionRunner(ctx, runner)
	}

	ctx.JSON(http.StatusOK, &res)
}

func GetRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.APIErrorNotFound(err)
		return
	}
	if !runner.Editable(ownerID, repoID) {
		ctx.APIErrorNotFound("No permission to get this runner")
		return
	}
	ctx.JSON(http.StatusOK, convert.ToActionRunner(ctx, runner))
}

func DeleteRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !runner.Editable(ownerID, repoID) {
		ctx.APIErrorNotFound("No permission to delete this runner")
		return
	}

	err = actions_model.DeleteRunner(ctx, runner.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
	}
	ctx.Status(http.StatusNoContent)
}
