// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"net/http"
	"net/url"
	"slices"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// ListRunnerGroups lists the runner groups visible to ownerID, 0 meaning the global ones
func ListRunnerGroups(ctx *context.APIContext, ownerID int64) {
	var repoID int64
	if name := ctx.FormString("visible_to_repository"); name != "" && ownerID != 0 {
		repo, err := repo_model.GetRepositoryByName(ctx, ownerID, name)
		if err != nil {
			ctx.APIErrorAuto(err)
			return
		}
		repoID = repo.ID
	}
	groups, err := actions_model.FindVisibleRunnerGroups(ctx, ownerID, repoID, 0)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	groups = slices.Insert(groups, 0, defaultRunnerGroup(ownerID))
	res := &api.ActionRunnerGroupsResponse{TotalCount: int64(len(groups)), Entries: make([]*api.ActionRunnerGroup, len(groups))}
	for i, group := range groups {
		res.Entries[i] = convert.ToActionRunnerGroup(group, ownerID, runnerGroupsURL(ctx, ownerID))
	}
	ctx.JSON(http.StatusOK, res)
}

func GetRunnerGroup(ctx *context.APIContext, ownerID int64) {
	if group := getVisibleRunnerGroup(ctx, ownerID); group != nil {
		ctx.JSON(http.StatusOK, convert.ToActionRunnerGroup(group, ownerID, runnerGroupsURL(ctx, ownerID)))
	}
}

func ListRunnerGroupRunners(ctx *context.APIContext, ownerID int64) {
	if group := getVisibleRunnerGroup(ctx, ownerID); group != nil {
		listRunners(ctx, &actions_model.FindRunnerOptions{
			OwnerID:       ownerID,
			WithAvailable: true,
			GroupID:       optional.Some(group.ID),
			ListOptions:   utils.GetListOptions(ctx),
		})
	}
}

// defaultRunnerGroup holds the ungrouped runners
func defaultRunnerGroup(ownerID int64) *actions_model.ActionRunnerGroup {
	return &actions_model.ActionRunnerGroup{OwnerID: ownerID, Name: "Default", IncludesAllRepositories: true}
}

func getVisibleRunnerGroup(ctx *context.APIContext, ownerID int64) *actions_model.ActionRunnerGroup {
	groupID := ctx.PathParamInt64("runner_group_id")
	if groupID == 0 {
		return defaultRunnerGroup(ownerID)
	}
	groups, err := actions_model.FindVisibleRunnerGroups(ctx, ownerID, 0, groupID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	if len(groups) == 0 {
		ctx.APIErrorNotFound()
		return nil
	}
	return groups[0]
}

func runnerGroupsURL(ctx *context.APIContext, ownerID int64) string {
	if ownerID == 0 {
		return setting.AppURL + "api/v1/admin/actions/runner-groups"
	}
	return setting.AppURL + "api/v1/orgs/" + url.PathEscape(ctx.Org.Organization.Name) + "/actions/runner-groups"
}
