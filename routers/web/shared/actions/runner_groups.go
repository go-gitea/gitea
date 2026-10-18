// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

func RunnerGroups(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}

	groups, err := actions_model.FindRunnerGroups(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("FindRunnerGroups", err)
		return
	}
	runners, repos, err := actions_model.CountRunnerGroupUsage(ctx, container.FilterSlice(groups, func(group *actions_model.ActionRunnerGroup) (int64, bool) {
		return group.ID, true
	}))
	if err != nil {
		ctx.ServerError("CountRunnerGroupUsage", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("actions.runners.groups")
	ctx.Data["PageType"] = "runner-groups"
	ctx.Data["PageIsSharedSettingsRunnerGroups"] = true
	ctx.Data["RunnerGroups"] = groups
	ctx.Data["RunnerGroupRunnerCounts"] = runners
	ctx.Data["RunnerGroupRepoCounts"] = repos
	ctx.HTML(http.StatusOK, rCtx.RunnersTemplate)
}

func RunnerGroupCreate(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return
	}
	group, err := actions_model.CreateRunnerGroup(ctx, rCtx.OwnerID, ctx.FormTrim("name"))
	switch {
	case err == nil:
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("actions.runners.groups.name_invalid"))
		return
	case errors.Is(err, util.ErrAlreadyExist):
		ctx.JSONError(ctx.Tr("actions.runners.groups.name_taken"))
		return
	default:
		ctx.ServerError("CreateRunnerGroup", err)
		return
	}
	ctx.JSONRedirect(fmt.Sprintf("%s/%d", rCtx.RunnerGroupsLink, group.ID))
}

func RunnerGroupEdit(ctx *context.Context) {
	rCtx, group := runnerGroupInScope(ctx)
	if group == nil {
		return
	}

	repos, err := actions_model.FindRunnerGroupRepos(ctx, group)
	if err != nil {
		ctx.ServerError("FindRunnerGroupRepos", err)
		return
	}
	candidates, err := actions_model.FindRunnerGroupCandidates(ctx, group.OwnerID)
	if err != nil {
		ctx.ServerError("FindRunnerGroupCandidates", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("actions.runners.groups")
	ctx.Data["PageType"] = "runner-group-edit"
	ctx.Data["PageIsSharedSettingsRunnerGroups"] = true
	ctx.Data["RunnerGroup"] = group
	ctx.Data["RunnerGroupRepos"] = repos
	ctx.Data["RunnerGroupCandidates"] = candidates
	ctx.Data["RunnerGroupMemberIDs"] = container.FilterSlice(candidates, func(runner *actions_model.ActionRunner) (int64, bool) {
		return runner.ID, runner.GroupID == group.ID
	})
	ctx.HTML(http.StatusOK, rCtx.RunnersTemplate)
}

func RunnerGroupEditPost(ctx *context.Context) {
	_, group := runnerGroupInScope(ctx)
	if group == nil {
		return
	}
	switch err := actions_model.UpdateRunnerGroup(ctx, group, ctx.FormString("repo_access") == "all", ctx.FormStringInt64s("repos"), ctx.FormStringInt64s("runners")); {
	case err == nil:
	case errors.Is(err, util.ErrPermissionDenied):
		ctx.JSONError(ctx.Tr("actions.runners.groups.target_invalid"))
		return
	default:
		ctx.ServerError("UpdateRunnerGroup", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.runners.groups.update_success"))
	ctx.JSONRedirect("")
}

func RunnerGroupDelete(ctx *context.Context) {
	rCtx, group := runnerGroupInScope(ctx)
	if group == nil {
		return
	}
	switch err := actions_model.DeleteRunnerGroup(ctx, group); {
	case err == nil:
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("actions.runners.groups.delete_not_empty"))
		return
	default:
		ctx.ServerError("DeleteRunnerGroup", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.runners.groups.delete_success"))
	ctx.JSONRedirect(rCtx.RunnerGroupsLink)
}

func runnerGroupInScope(ctx *context.Context) (*runnersCtx, *actions_model.ActionRunnerGroup) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return nil, nil
	}
	group, _, err := db.GetByID[actions_model.ActionRunnerGroup](ctx, ctx.PathParamInt64("groupid"))
	if err != nil {
		ctx.ServerError("GetByID", err)
		return nil, nil
	}
	if group == nil || group.OwnerID != rCtx.OwnerID {
		ctx.NotFound(util.NewPermissionDeniedErrorf("no permission to edit this runner group"))
		return nil, nil
	}
	return rCtx, group
}
