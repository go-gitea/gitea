// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	stdctx "context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

type runnerGroupView struct {
	*actions_model.ActionRunnerGroup
	Repos   int64
	Runners int64
}

func getRunnerGroupsCtx(ctx *context.Context) *runnersCtx {
	rCtx, err := getRunnersCtx(ctx)
	if err != nil {
		ctx.ServerError("getRunnersCtx", err)
		return nil
	}
	if rCtx.IsRepo {
		ctx.NotFound(errors.New("runner groups are not available for repositories"))
		return nil
	}
	rCtx.RedirectLink = strings.TrimSuffix(rCtx.RedirectLink, "runners/") + "runner-groups"
	return rCtx
}

func RunnerGroups(ctx *context.Context) {
	rCtx := getRunnerGroupsCtx(ctx)
	if rCtx == nil {
		return
	}

	groups, err := actions_model.FindRunnerGroups(ctx, rCtx.OwnerID)
	if err != nil {
		ctx.ServerError("FindRunnerGroups", err)
		return
	}
	runners, repos, err := actions_model.CountRunnerGroupUsage(ctx, container.FilterSlice(groups, func(g *actions_model.ActionRunnerGroup) (int64, bool) {
		return g.ID, true
	}))
	if err != nil {
		ctx.ServerError("CountRunnerGroupUsage", err)
		return
	}
	views := make([]*runnerGroupView, 0, len(groups))
	for _, group := range groups {
		views = append(views, &runnerGroupView{ActionRunnerGroup: group, Runners: runners[group.ID], Repos: repos[group.ID]})
	}

	ctx.Data["Title"] = ctx.Tr("actions.runners.groups")
	ctx.Data["PageType"] = "runner-groups"
	ctx.Data["PageIsSharedSettingsRunnerGroups"] = true
	ctx.Data["Link"] = rCtx.RedirectLink
	ctx.Data["RunnerGroups"] = views
	ctx.HTML(http.StatusOK, rCtx.RunnersTemplate)
}

func RunnerGroupCreate(ctx *context.Context) {
	rCtx := getRunnerGroupsCtx(ctx)
	if rCtx == nil {
		return
	}
	name := ctx.FormTrim("name")
	if name == "" || utf8.RuneCountInString(name) > 255 {
		ctx.JSONError(ctx.Tr("actions.runners.groups.name_invalid"))
		return
	}
	group, err := actions_model.CreateRunnerGroup(ctx, rCtx.OwnerID, name)
	switch {
	case err == nil:
	case errors.Is(err, util.ErrAlreadyExist):
		ctx.JSONError(ctx.Tr("actions.runners.groups.name_taken"))
		return
	default:
		ctx.ServerError("CreateRunnerGroup", err)
		return
	}
	ctx.JSONRedirect(fmt.Sprintf("%s/%d", rCtx.RedirectLink, group.ID))
}

func RunnerGroupEdit(ctx *context.Context) {
	rCtx, group := runnerGroupInScope(ctx)
	if group == nil {
		return
	}

	repos, err := actions_model.FindRunnerGroupRepos(ctx, group.ID)
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
	ctx.Data["Link"] = fmt.Sprintf("%s/%d", rCtx.RedirectLink, group.ID)
	ctx.Data["RunnerGroup"] = group
	ctx.Data["RunnerGroupRepos"] = repos
	ctx.Data["RunnerGroupCandidates"] = candidates
	ctx.Data["RunnerGroupMemberIDs"] = base.Int64sToStrings(container.FilterSlice(candidates, func(runner *actions_model.ActionRunner) (int64, bool) {
		return runner.ID, runner.GroupID == group.ID
	}))
	ctx.Data["RunnerLink"] = strings.TrimSuffix(rCtx.RedirectLink, "runner-groups") + "runners"
	ctx.HTML(http.StatusOK, rCtx.RunnersTemplate)
}

func RunnerGroupEditPost(ctx *context.Context) {
	rCtx, group := runnerGroupInScope(ctx)
	if group == nil {
		return
	}
	repoIDs, err := base.StringsToInt64s(util.SplitTrimSpace(ctx.FormString("repos"), ","))
	runnerIDs, errRunners := base.StringsToInt64s(util.SplitTrimSpace(ctx.FormString("runners"), ","))
	err = errors.Join(err, errRunners)
	if err == nil {
		err = db.WithTx(ctx, func(txCtx stdctx.Context) error {
			if err := actions_model.SetRunnerAccess(txCtx, group, repoIDs); err != nil {
				return err
			}
			return actions_model.SetRunnerGroupMembers(txCtx, group, runnerIDs)
		})
	}
	switch {
	case err == nil:
		ctx.Flash.Success(ctx.Tr("actions.runners.groups.update_success"))
	case errors.Is(err, util.ErrPermissionDenied), errors.Is(err, strconv.ErrSyntax), errors.Is(err, strconv.ErrRange):
		ctx.Flash.Error(ctx.Tr("actions.runners.groups.target_invalid"))
	default:
		ctx.ServerError("RunnerGroupEditPost", err)
		return
	}
	ctx.Redirect(fmt.Sprintf("%s/%d", rCtx.RedirectLink, group.ID))
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
	ctx.JSONRedirect(rCtx.RedirectLink)
}

func runnerGroupInScope(ctx *context.Context) (*runnersCtx, *actions_model.ActionRunnerGroup) {
	rCtx := getRunnerGroupsCtx(ctx)
	if rCtx == nil {
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
