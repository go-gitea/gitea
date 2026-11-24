// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

func RunnerGroups(ctx *context.Context) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	groups, err := actions_model.FindRunnerGroups(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	runners, repos, err := actions_model.CountRunnerGroupUsage(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}
	candidates, err := actions_model.FindRunnerGroupCandidates(ctx, group.OwnerID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if err := actions_model.RunnerList(candidates).LoadGroups(ctx); err != nil {
		ctx.ServerErrorAuto(err)
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
	repoIDs, repoErr := base.StringsToInt64s(strings.Split(ctx.FormString("repos"), ","))
	runnerIDs, runnerErr := base.StringsToInt64s(strings.Split(ctx.FormString("runners"), ","))
	if err := errors.Join(repoErr, runnerErr); err != nil {
		ctx.HTTPError(http.StatusBadRequest, err.Error())
		return
	}
	switch err := actions_model.UpdateRunnerGroup(ctx, group, ctx.FormString("repo_access") == "all", repoIDs, runnerIDs); {
	case err == nil:
	case errors.Is(err, util.ErrPermissionDenied):
		ctx.JSONError(ctx.Tr("actions.runners.groups.target_invalid"))
		return
	default:
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.runners.groups.delete_success"))
	ctx.JSONRedirect(rCtx.RunnerGroupsLink)
}

func runnerGroupInScope(ctx *context.Context) (*runnersCtx, *actions_model.ActionRunnerGroup) {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return nil, nil
	}
	group, _, err := db.GetByID[actions_model.ActionRunnerGroup](ctx, ctx.PathParamInt64("groupid"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return nil, nil
	}
	if group == nil || group.OwnerID != rCtx.OwnerID {
		ctx.NotFound()
		return nil, nil
	}
	return rCtx, group
}
