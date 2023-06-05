// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplMembers template for organization members page
	tplMembers base.TplName = "org/member/members"
)

// Members render organization users page
func Members(ctx *context.Context) {
	org := ctx.Org.Organization
	ctx.Data["Title"] = org.FullName
	ctx.Data["PageIsOrgMembers"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := &organization.FindOrgMembersOpts{
		OrgID:      org.ID,
		PublicOnly: true,
	}

	if ctx.Doer != nil {
		isMember, err := ctx.Org.Organization.IsOrgMember(ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember")
			return
		}
		opts.PublicOnly = !isMember && !ctx.Doer.IsAdmin
	}
	ctx.Data["PublicOnly"] = opts.PublicOnly

	total, err := organization.CountOrgMembers(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountOrgMembers")
		return
	}

	pager := context.NewPagination(int(total), setting.UI.MembersPagingNum, page, 5)
	opts.ListOptions.Page = page
	opts.ListOptions.PageSize = setting.UI.MembersPagingNum
	members, membersIsPublic, err := organization.FindOrgMembers(opts)
	if err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	ctx.Data["Page"] = pager
	ctx.Data["Members"] = members
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["MembersIsPublicMember"] = membersIsPublic
	ctx.Data["MembersIsUserOrgOwner"] = organization.IsUserOrgOwner(members, org.ID)
	ctx.Data["MembersTwoFaStatus"] = members.GetTwoFaStatus()

	ctx.HTML(http.StatusOK, tplMembers)
}

// MembersAction response for operation to a member of organization
func MembersAction(ctx *context.Context) {
	uid := ctx.FormInt64("uid")
	if uid == 0 {
		ctx.Redirect(ctx.Org.OrgLink + "/members")
		return
	}

	org := ctx.Org.Organization
	var err error
	switch ctx.Params(":action") {
	case "private":
		if ctx.Doer.ID != uid && !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = organization.ChangeOrgUserStatus(org.ID, uid, false)
	case "public":
		if ctx.Doer.ID != uid && !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = organization.ChangeOrgUserStatus(org.ID, uid, true)
	case "remove":
		if !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = models.RemoveOrgUser(org.ID, uid)
		if organization.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"redirect": ctx.Org.OrgLink + "/members",
			})
			return
		}
	case "leave":
		err = models.RemoveOrgUser(org.ID, ctx.Doer.ID)
		if err == nil {
			ctx.Flash.Success(ctx.Tr("form.organization_leave_success", org.DisplayName()))
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"redirect": "", // keep the user stay on current page, in case they want to do other operations.
			})
		} else if organization.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"redirect": ctx.Org.OrgLink + "/members",
			})
		} else {
			log.Error("RemoveOrgUser(%d,%d): %v", org.ID, ctx.Doer.ID, err)
		}
		return
	}

	if err != nil {
		log.Error("Action(%s): %v", ctx.Params(":action"), err)
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"ok":  false,
			"err": err.Error(),
		})
		return
	}

	redirect := ctx.Org.OrgLink + "/members"
	if ctx.Params(":action") == "leave" {
		redirect = setting.AppSubURL + "/"
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": redirect,
	})
}
