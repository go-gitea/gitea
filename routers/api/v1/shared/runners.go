// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
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

// ListRunners lists runners for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all runners including global runners, does not appear in sql where clause
// ownerID == 0 and repoID != 0 means all runners for the given repo
// ownerID != 0 and repoID == 0 means all runners for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func ListRunners(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	opts := &actions_model.FindRunnerOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: utils.GetListOptions(ctx),
	}
	opts.IsDisabled = ctx.FormOptionalBool("disabled")
	runners, total, err := db.FindAndCount[actions_model.ActionRunner](ctx, opts)
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

func getRunnerByID(ctx *context.APIContext, ownerID, repoID, runnerID int64) (*actions_model.ActionRunner, bool) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}

	runner, err := actions_model.GetRunnerByID(ctx, runnerID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil, false
	}

	if !runner.EditableInContext(ownerID, repoID) {
		ctx.APIErrorNotFound("No permission to access this runner")
		return nil, false
	}
	return runner, true
}

// GetRunner get the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func GetRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		setting.PanicInDevOrTesting("ownerID and repoID should not be both set")
	}
	runner, ok := getRunnerByID(ctx, ownerID, repoID, runnerID)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToActionRunner(ctx, runner))
}

// DeleteRunner deletes the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func DeleteRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	runner, ok := getRunnerByID(ctx, ownerID, repoID, runnerID)
	if !ok {
		return
	}

	err := actions_model.DeleteRunner(ctx, runner.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func UpdateRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	runner, ok := getRunnerByID(ctx, ownerID, repoID, runnerID)
	if !ok {
		return
	}

	form := web.GetForm(ctx).(*api.EditActionRunnerOption)
	if form.Disabled == nil {
		ctx.APIError(http.StatusUnprocessableEntity, "[Disabled]: Required")
		return
	}

	if err := actions_model.SetRunnerDisabled(ctx, runner, *form.Disabled); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	GetRunner(ctx, ownerID, repoID, runnerID)
}
