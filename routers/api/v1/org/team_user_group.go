// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/organization"
	usergroup_model "code.gitea.io/gitea/models/usergroup"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	org_service "code.gitea.io/gitea/services/org"
)

// ListTeamUserGroups lists user groups assigned to a team.
func ListTeamUserGroups(ctx *context.APIContext) {
	groups, err := organization.GetTeamUserGroups(ctx, ctx.Org.Team.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiGroups := make([]*api.UserGroup, len(groups))
	for i, group := range groups {
		apiGroups[i] = convert.ToUserGroup(group)
	}
	ctx.JSON(http.StatusOK, apiGroups)
}

// AddTeamUserGroup assigns a user group to the team.
func AddTeamUserGroup(ctx *context.APIContext) {
	groupID := ctx.PathParamInt64("group_id")
	group, err := usergroup_model.GetUserGroupByID(ctx, groupID)
	if err != nil {
		if usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := org_service.AddTeamUserGroup(ctx, ctx.Org.Team, group.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToUserGroup(group))
}

// RemoveTeamUserGroup removes a user group from the team.
func RemoveTeamUserGroup(ctx *context.APIContext) {
	groupID := ctx.PathParamInt64("group_id")
	if err := org_service.RemoveTeamUserGroup(ctx, ctx.Org.Team, groupID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
