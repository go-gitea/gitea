// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"net/http"

	"gitea.dev/models/db"
	group_model "gitea.dev/models/group"
	"gitea.dev/models/organization"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	group_service "gitea.dev/services/group"
)

const tplGroupNew = "group/create"

func NewGroup(ctx *context.Context) {
	var owner *user_model.User
	if ctx.Org.Organization != nil {
		ctx.Data["Title"] = ctx.Org.Organization.FullName
		owner = ctx.Org.Organization.AsUser()
	} else {
		owner = ctx.Doer
	}
	ctx.Data["PageIsNewGroup"] = true
	if ctx.RepoGroup.Group != nil {
		ctx.Data["Group"] = &group_model.Group{ParentGroupID: ctx.RepoGroup.Group.ID}
	} else {
		ctx.Data["Group"] = &group_model.Group{}
	}
	ctx.Data["Units"] = unit_model.Units
	orgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return
	}

	var orgsAvailable []*organization.Organization
	for i := range orgs {
		if ctx.Doer.CanCreateRepoIn(orgs[i].AsUser()) {
			orgsAvailable = append(orgsAvailable, orgs[i])
		}
	}
	ctx.Data["Orgs"] = orgsAvailable

	opts := group_model.FindGroupsOptions{
		ActorID: ctx.Doer.ID,
		OwnerID: owner.ID,
	}
	cond := group_model.AccessibleGroupCondition(ctx.Doer)
	cond = cond.And(opts.ToConds())
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		ParentGroupID: -1,
	}, cond)
	for _, g := range groups {
		err = g.LoadAccessibleSubgroups(ctx, true, ctx.Doer, false)
		if err != nil {
			ctx.ServerError("LoadAccessibleSubgroups", err)
			return
		}
	}
	if err != nil {
		ctx.ServerError("FindGroupsByCond", err)
		return
	}
	ctx.Data["Groups"] = groups
	ctx.HTML(http.StatusOK, tplGroupNew)
}

func NewGroupPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateGroupForm)
	var owner *user_model.User
	if ctx.Org.Organization != nil {
		ctx.Data["Title"] = ctx.Org.Organization.FullName
		owner = ctx.Org.Organization.AsUser()
	} else {
		owner = ctx.Doer
	}
	g := &group_model.Group{
		OwnerID:       owner.ID,
		Name:          form.GroupName,
		Description:   form.Description,
		OwnerName:     owner.Name,
		ParentGroupID: form.ParentGroupID,
	}
	ctx.Data["PageIsGroupNew"] = true
	ctx.Data["Units"] = unit_model.Units
	ctx.Data["Group"] = g

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplGroupNew)
		return
	}

	if err := group_service.NewGroup(ctx, g, ctx.Doer); err != nil {
		ctx.Data["Err_GroupName"] = true
		ctx.ServerError("NewGroup", err)
		return
	}
	log.Trace("Group created: %s/%s", owner.Name, g.Name)
	ctx.Redirect(g.GroupLink())
}
