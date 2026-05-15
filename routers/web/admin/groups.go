// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/usergroup"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	org_service "code.gitea.io/gitea/services/org"
)

const (
	tplAdminUserGroups    templates.TplName = "admin/user_group/list"
	tplAdminUserGroupEdit templates.TplName = "admin/user_group/edit"
	tplAdminUserGroupNew  templates.TplName = "admin/user_group/new"
)

type globalGroupListItem struct {
	Group      *usergroup.UserGroup
	Parent     *usergroup.UserGroup
	NumMembers int64
	NumTeams   int64
}

type globalGroupCount struct {
	GroupID int64 `xorm:"group_id"`
	Count   int64 `xorm:"count"`
}

// buildAllGroupPaths converts a slice of groups into an id → full display path map
// (e.g. 42 → "Engineering / Backend / API") for use in searchable dropdowns.
func buildAllGroupPaths(ctx *context.Context, groups []*usergroup.UserGroup) (map[int64]string, error) {
	ids := make([]int64, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	return usergroup.GetUserGroupFullPaths(ctx, ids)
}

// UserGroups shows all user groups.
func UserGroups(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.user_groups")
	ctx.Data["PageIsAdminUserGroups"] = true

	opts := db.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.FormInt("page"),
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	groups, total, err := usergroup.SearchUserGroups(ctx, &usergroup.SearchUserGroupOptions{
		ListOptions: opts,
		Keyword:     keyword,
	})
	if err != nil {
		ctx.ServerError("SearchUserGroups", err)
		return
	}

	groupIDs := make([]int64, 0, len(groups))
	parentIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
		if group.ParentID > 0 {
			parentIDs = append(parentIDs, group.ParentID)
		}
	}

	parentMap, err := usergroup.GetUserGroupsByIDs(ctx, parentIDs)
	if err != nil {
		ctx.ServerError("GetUserGroupsByIDs", err)
		return
	}

	memberCounts := make(map[int64]int64)
	if len(groupIDs) > 0 {
		var counts []globalGroupCount
		if err := db.GetEngine(ctx).Table("user_group_member").
			In("group_id", groupIDs).
			Select("group_id, COUNT(*) AS count").
			GroupBy("group_id").
			Find(&counts); err != nil {
			ctx.ServerError("CountUserGroupMembers", err)
			return
		}
		for _, count := range counts {
			memberCounts[count.GroupID] = count.Count
		}
	}

	teamCounts := make(map[int64]int64)
	if len(groupIDs) > 0 {
		var counts []globalGroupCount
		if err := db.GetEngine(ctx).Table("team_user_group").
			In("group_id", groupIDs).
			Select("group_id, COUNT(*) AS count").
			GroupBy("group_id").
			Find(&counts); err != nil {
			ctx.ServerError("CountTeamUserGroups", err)
			return
		}
		for _, count := range counts {
			teamCounts[count.GroupID] = count.Count
		}
	}

	items := make([]globalGroupListItem, 0, len(groups))
	for _, group := range groups {
		items = append(items, globalGroupListItem{
			Group:      group,
			Parent:     parentMap[group.ParentID],
			NumMembers: memberCounts[group.ID],
			NumTeams:   teamCounts[group.ID],
		})
	}

	var allGroups []*usergroup.UserGroup
	if err := db.GetEngine(ctx).OrderBy("lower_name").Find(&allGroups); err != nil {
		ctx.ServerError("ListAllUserGroups", err)
		return
	}

	ctx.Data["Groups"] = items
	ctx.Data["AllGroups"] = allGroups
	ctx.Data["CurrentPage"] = opts.Page
	ctx.Data["Total"] = total
	pager := context.NewPagination(total, opts.PageSize, opts.Page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplAdminUserGroups)
}

// UserGroupNew renders the new user group creation form.
func UserGroupNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.user_groups")
	ctx.Data["PageIsAdminUserGroups"] = true

	var allGroups []*usergroup.UserGroup
	if err := db.GetEngine(ctx).OrderBy("lower_name").Find(&allGroups); err != nil {
		ctx.ServerError("ListAllUserGroups", err)
		return
	}
	ctx.Data["AllGroups"] = allGroups

	allGroupPaths, err := buildAllGroupPaths(ctx, allGroups)
	if err != nil {
		ctx.ServerError("BuildAllGroupPaths", err)
		return
	}
	ctx.Data["AllGroupPaths"] = allGroupPaths
	// Pre-select a parent when arriving via the "New child group" link.
	ctx.Data["PreselectedParentID"] = ctx.FormInt64("parent_id")
	ctx.HTML(http.StatusOK, tplAdminUserGroupNew)
}

// UserGroupNewPost creates a user group and redirects to its edit page.
func UserGroupNewPost(ctx *context.Context) {
	name := ctx.FormTrim("name")
	slug := ctx.FormTrim("slug")
	description := ctx.FormTrim("description")
	parentID := ctx.FormInt64("parent_id")

	group := &usergroup.UserGroup{
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentID,
	}
	if err := usergroup.CreateUserGroup(ctx, group); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.user_groups.create_failed", err))
		ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/new")
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.create_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups")
}

// UserGroupEdit shows a user group edit page.
func UserGroupEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.user_groups")
	ctx.Data["PageIsAdminUserGroups"] = true

	groupID := ctx.PathParamInt64("groupid")
	group, err := usergroup.GetUserGroupByID(ctx, groupID)
	if err != nil {
		ctx.ServerError("GetUserGroupByID", err)
		return
	}

	var groups []*usergroup.UserGroup
	if err := db.GetEngine(ctx).OrderBy("lower_name").Find(&groups); err != nil {
		ctx.ServerError("ListUserGroups", err)
		return
	}

	descendantIDs, err := usergroup.ExpandUserGroupIDsToDescendants(ctx, []int64{groupID})
	if err != nil {
		ctx.ServerError("ExpandUserGroupIDsToDescendants", err)
		return
	}
	descendantSet := make(map[int64]struct{}, len(descendantIDs))
	for _, id := range descendantIDs {
		descendantSet[id] = struct{}{}
	}
	eligibleParentGroups := make([]*usergroup.UserGroup, 0, len(groups))
	for _, candidate := range groups {
		if _, blocked := descendantSet[candidate.ID]; blocked {
			continue
		}
		eligibleParentGroups = append(eligibleParentGroups, candidate)
	}

	// Count members first so we can build a paginator.
	childGroups := make([]*usergroup.UserGroup, 0, len(groups))
	countGroupIDs := []int64{groupID}
	for _, candidate := range groups {
		if candidate.ParentID == groupID {
			childGroups = append(childGroups, candidate)
			countGroupIDs = append(countGroupIDs, candidate.ID)
		}
	}

	memberCountMap, err := usergroup.GetUserGroupMemberCounts(ctx, countGroupIDs)
	if err != nil {
		ctx.ServerError("GetUserGroupMemberCounts", err)
		return
	}
	totalMembers := memberCountMap[groupID]
	totalChildGroups := int64(len(childGroups))

	page := max(ctx.FormInt("page"), 1)
	childPage := max(ctx.FormInt("child_page"), 1)
	teamPage := max(ctx.FormInt("team_page"), 1)
	pageSize := setting.UI.Admin.UserPagingNum
	members, err := usergroup.GetUserGroupMembers(ctx, groupID, db.ListOptions{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ctx.ServerError("GetUserGroupMembers", err)
		return
	}

	teamWithOrgs, err := organization.GetTeamsWithOrgByUserGroupID(ctx, groupID)
	if err != nil {
		ctx.ServerError("GetTeamsWithOrgByUserGroupID", err)
		return
	}
	totalTeams := int64(len(teamWithOrgs))

	childStart := (childPage - 1) * pageSize
	if childStart < len(childGroups) {
		childEnd := min(childStart+pageSize, len(childGroups))
		childGroups = childGroups[childStart:childEnd]
	} else {
		childGroups = nil
	}

	teamStart := (teamPage - 1) * pageSize
	if teamStart < len(teamWithOrgs) {
		teamEnd := min(teamStart+pageSize, len(teamWithOrgs))
		teamWithOrgs = teamWithOrgs[teamStart:teamEnd]
	} else {
		teamWithOrgs = nil
	}

	ctx.Data["Group"] = group
	ctx.Data["Groups"] = groups
	ctx.Data["EligibleParentGroups"] = eligibleParentGroups
	ctx.Data["ChildGroups"] = childGroups
	ctx.Data["ChildGroupsTotal"] = totalChildGroups
	ctx.Data["ChildGroupMemberCounts"] = memberCountMap
	ctx.Data["Members"] = members
	ctx.Data["TeamWithOrgs"] = teamWithOrgs
	ctx.Data["TeamWithOrgsTotal"] = totalTeams
	ctx.Data["MembersTotal"] = totalMembers
	pager := context.NewPagination(totalMembers, pageSize, page, 5)
	ctx.Data["Page"] = pager
	childPager := context.NewPagination(totalChildGroups, pageSize, childPage, 5)
	childPager.AddParamFromRequest(ctx.Req)
	childPager.RemoveParam(container.Set[string]{"child_page": {}})
	ctx.Data["ChildPage"] = childPager
	teamPager := context.NewPagination(totalTeams, pageSize, teamPage, 5)
	teamPager.AddParamFromRequest(ctx.Req)
	teamPager.RemoveParam(container.Set[string]{"team_page": {}})
	ctx.Data["TeamPage"] = teamPager

	allGroupPaths, err := buildAllGroupPaths(ctx, groups)
	if err != nil {
		ctx.ServerError("BuildAllGroupPaths", err)
		return
	}
	ctx.Data["AllGroupPaths"] = allGroupPaths
	ctx.HTML(http.StatusOK, tplAdminUserGroupEdit)
}

// UserGroupEditPost updates a user group.
func UserGroupEditPost(ctx *context.Context) {
	groupID := ctx.PathParamInt64("groupid")
	group, err := usergroup.GetUserGroupByID(ctx, groupID)
	if err != nil {
		ctx.ServerError("GetUserGroupByID", err)
		return
	}

	group.Name = ctx.FormTrim("name")
	group.Slug = ctx.FormTrim("slug")
	group.Description = ctx.FormTrim("description")
	group.ParentID = ctx.FormInt64("parent_id")
	if err := org_service.UpdateUserGroupWithSync(ctx, group); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.user_groups.update_failed", err))
		ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.update_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
}

// UserGroupDelete deletes a user group.
func UserGroupDelete(ctx *context.Context) {
	groupID := ctx.PathParamInt64("groupid")

	if err := org_service.DeleteUserGroupWithSync(ctx, groupID); err != nil {
		if usergroup.IsErrUserGroupNotExist(err) || usergroup.IsErrUserGroupHasChildren(err) {
			ctx.Flash.Error(ctx.Tr("admin.user_groups.delete_failed", err))
			ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
			return
		}
		ctx.ServerError("DeleteUserGroupWithSync", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.delete_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups")
}

// UserGroupAddMember adds a user to a user group.
func UserGroupAddMember(ctx *context.Context) {
	groupID := ctx.PathParamInt64("groupid")
	userName := ctx.FormTrim("uname")
	user, err := user_model.GetUserByName(ctx, userName)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.user_groups.add_member_failed", userName))
		ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
		return
	}

	isMember, err := usergroup.IsUserInUserGroup(ctx, groupID, user.ID)
	if err != nil {
		ctx.ServerError("IsUserInUserGroup", err)
		return
	}
	if !isMember {
		if err := usergroup.AddUserToUserGroup(ctx, groupID, user.ID); err != nil {
			ctx.Flash.Error(ctx.Tr("admin.user_groups.add_member_failed", userName))
			ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
			return
		}
	}

	if err := org_service.RecalculateUserGroupTeamAccesses(ctx, groupID); err != nil {
		ctx.ServerError("RecalculateUserGroupTeamAccesses", err)
		return
	}

	// Add the user to all orgs whose teams reference this group (via ancestors).
	if err := org_service.SyncGroupMemberToOrgs(ctx, groupID, user.ID, true); err != nil {
		ctx.ServerError("SyncGroupMemberToOrgs", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.add_member_success", userName))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
}

// UserGroupRemoveMember removes a user from a user group.
func UserGroupRemoveMember(ctx *context.Context) {
	groupID := ctx.PathParamInt64("groupid")
	userID := ctx.FormInt64("uid")

	if err := usergroup.RemoveUserFromUserGroup(ctx, groupID, userID); err != nil {
		ctx.ServerError("RemoveUserFromUserGroup", err)
		return
	}

	if err := org_service.RecalculateUserGroupTeamAccesses(ctx, groupID); err != nil {
		ctx.ServerError("RecalculateUserGroupTeamAccesses", err)
		return
	}

	// Reevaluate org membership for the removed user.
	if err := org_service.SyncGroupMemberToOrgs(ctx, groupID, userID, false); err != nil {
		ctx.ServerError("SyncGroupMemberToOrgs", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.remove_member_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
}

// UserGroupRemoveFromTeam removes the group from a specific org team.
func UserGroupRemoveFromTeam(ctx *context.Context) {
	groupID := ctx.PathParamInt64("groupid")
	teamID := ctx.FormInt64("team_id")

	team, err := organization.GetTeamByID(ctx, teamID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.user_groups.remove_from_team_failed"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
		return
	}

	if err := org_service.RemoveTeamUserGroup(ctx, team, groupID); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.user_groups.remove_from_team_failed"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.user_groups.remove_from_team_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/user-groups/" + ctx.PathParam("groupid"))
}
