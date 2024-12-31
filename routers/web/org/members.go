// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	org_service "code.gitea.io/gitea/services/org"
)

const (
	// tplMembers template for organization members page
	tplMembers templates.TplName = "org/member/members"
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
		Doer:  ctx.Doer,
		OrgID: org.ID,
	}

	if ctx.Doer != nil {
		isMember, err := ctx.Org.Organization.IsOrgMember(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember")
			return
		}
		opts.IsDoerMember = isMember
	}
	ctx.Data["PublicOnly"] = opts.PublicOnly()

	total, err := organization.CountOrgMembers(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountOrgMembers")
		return
	}

	_, err = shared_user.PrepareOrgHeader(ctx)
	if err != nil {
		ctx.ServerError("PrepareOrgHeader", err)
		return
	}

	pager := context.NewPagination(int(total), setting.UI.MembersPagingNum, page, 5)
	opts.ListOptions.Page = page
	opts.ListOptions.PageSize = setting.UI.MembersPagingNum
	members, membersIsPublic, err := organization.FindOrgMembers(ctx, opts)
	if err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	ctx.Data["Page"] = pager
	ctx.Data["Members"] = members
	ctx.Data["MembersIsPublicMember"] = membersIsPublic
	ctx.Data["MembersIsUserOrgOwner"] = organization.IsUserOrgOwner(ctx, members, org.ID)
	ctx.Data["MembersTwoFaStatus"] = members.GetTwoFaStatus(ctx)

	ctx.HTML(http.StatusOK, tplMembers)
}

// MembersAction response for operation to a member of organization
func MembersAction(ctx *context.Context) {
	member, err := user_model.GetUserByID(ctx, ctx.FormInt64("uid"))
	if err != nil {
		log.Error("GetUserByID: %v", err)
	}
	if member == nil {
		ctx.Redirect(ctx.Org.OrgLink + "/members")
		return
	}

	org := ctx.Org.Organization

	switch ctx.PathParam("action") {
	case "private":
		if ctx.Doer.ID != member.ID && !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = organization.ChangeOrgUserStatus(ctx, org.ID, member.ID, false)
	case "public":
		if ctx.Doer.ID != member.ID && !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = organization.ChangeOrgUserStatus(ctx, org.ID, member.ID, true)
	case "remove":
		if !ctx.Org.IsOwner {
			ctx.Error(http.StatusNotFound)
			return
		}
		err = org_service.RemoveOrgUser(ctx, org, member)
		if organization.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.JSONRedirect(ctx.Org.OrgLink + "/members")
			return
		}
	case "leave":
		err = org_service.RemoveOrgUser(ctx, org, ctx.Doer)
		if err == nil {
			ctx.Flash.Success(ctx.Tr("form.organization_leave_success", org.DisplayName()))
			ctx.JSON(http.StatusOK, map[string]any{
				"redirect": "", // keep the user stay on current page, in case they want to do other operations.
			})
		} else if organization.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.JSONRedirect(ctx.Org.OrgLink + "/members")
		} else {
			log.Error("RemoveOrgUser(%d,%d): %v", org.ID, ctx.Doer.ID, err)
		}
		return
	}

	if err != nil {
		log.Error("Action(%s): %v", ctx.PathParam("action"), err)
		ctx.JSON(http.StatusOK, map[string]any{
			"ok":  false,
			"err": err.Error(),
		})
		return
	}

	redirect := ctx.Org.OrgLink + "/members"
	if ctx.PathParam("action") == "leave" {
		redirect = setting.AppSubURL + "/"
	}

	ctx.JSONRedirect(redirect)
}
