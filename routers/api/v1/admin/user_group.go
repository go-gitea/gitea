// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	user_model "gitea.dev/models/user"
	usergroup_model "gitea.dev/models/usergroup"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	org_service "gitea.dev/services/org"
)

// ListUserGroups lists all user groups.
func ListUserGroups(ctx *context.APIContext) {
	listOptions := utils.GetListOptions(ctx)
	keyword := ctx.FormTrim("q")

	groups, total, err := usergroup_model.SearchUserGroups(ctx, &usergroup_model.SearchUserGroupOptions{
		ListOptions: listOptions,
		Keyword:     keyword,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiGroups := make([]*api.UserGroup, len(groups))
	for i, group := range groups {
		apiGroups[i] = convert.ToUserGroup(group)
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, apiGroups)
}

// CreateUserGroup creates a new user group.
func CreateUserGroup(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateUserGroupOption)
	group := &usergroup_model.UserGroup{
		Name:     form.Name,
		ParentID: form.ParentID,
	}

	if err := usergroup_model.CreateUserGroup(ctx, group); err != nil {
		if usergroup_model.IsErrUserGroupAlreadyExist(err) || usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToUserGroup(group))
}

// EditUserGroup updates an existing user group.
func EditUserGroup(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.EditUserGroupOption)
	groupID := ctx.PathParamInt64("id")

	group, err := usergroup_model.GetUserGroupByID(ctx, groupID)
	if err != nil {
		if usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	group.Name = form.Name
	group.ParentID = form.ParentID
	if err := org_service.UpdateUserGroupWithSync(ctx, group); err != nil {
		if usergroup_model.IsErrUserGroupAlreadyExist(err) || usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToUserGroup(group))
}

// DeleteUserGroup deletes a user group.
func DeleteUserGroup(ctx *context.APIContext) {
	groupID := ctx.PathParamInt64("id")

	if err := org_service.DeleteUserGroupWithSync(ctx, groupID); err != nil {
		switch {
		case usergroup_model.IsErrUserGroupNotExist(err):
			ctx.APIErrorNotFound(err)
		case usergroup_model.IsErrUserGroupHasChildren(err):
			ctx.APIError(http.StatusUnprocessableEntity, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListUserGroupMembers lists direct members of a user group.
func ListUserGroupMembers(ctx *context.APIContext) {
	groupID := ctx.PathParamInt64("id")
	listOptions := utils.GetListOptions(ctx)

	if _, err := usergroup_model.GetUserGroupByID(ctx, groupID); err != nil {
		if usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	members, err := usergroup_model.GetUserGroupMembers(ctx, groupID, listOptions)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiUsers := make([]*api.User, len(members))
	for i, user := range members {
		apiUsers[i] = convert.ToUser(ctx, user, nil)
	}

	ctx.JSON(http.StatusOK, apiUsers)
}

// ReplaceUserGroupMembers replaces the direct members of a user group.
func ReplaceUserGroupMembers(ctx *context.APIContext) {
	groupID := ctx.PathParamInt64("id")
	form := web.GetForm(ctx).(*api.UserGroupMembersOption)

	if _, err := usergroup_model.GetUserGroupByID(ctx, groupID); err != nil {
		if usergroup_model.IsErrUserGroupNotExist(err) {
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := org_service.SyncReplaceUserGroupMembers(ctx, groupID, form.UserIDs); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	users, err := user_model.GetUsersByIDs(ctx, form.UserIDs)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiUsers := make([]*api.User, len(users))
	for i, user := range users {
		apiUsers[i] = convert.ToUser(ctx, user, nil)
	}
	ctx.JSON(http.StatusOK, apiUsers)
}
