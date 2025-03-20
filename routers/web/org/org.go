// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const (
	// tplCreateOrg template path for create organization
	tplCreateOrg templates.TplName = "org/create"
)

// Create render the page for create organization
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_org")
	ctx.Data["DefaultOrgVisibilityMode"] = setting.Service.DefaultOrgVisibilityMode
	if !ctx.Doer.CanCreateOrganization() {
		ctx.ServerError("Not allowed", errors.New(ctx.Locale.TrString("org.form.create_org_not_allowed")))
		return
	}
	ctx.HTML(http.StatusOK, tplCreateOrg)
}

// CreatePost response for create organization
func CreatePost(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.CreateOrgForm)
	ctx.Data["Title"] = ctx.Tr("new_org")

	if !ctx.Doer.CanCreateOrganization() {
		ctx.ServerError("Not allowed", errors.New(ctx.Locale.TrString("org.form.create_org_not_allowed")))
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplCreateOrg)
		return
	}

	org := &organization.Organization{
		Name:                      form.OrgName,
		IsActive:                  true,
		Type:                      user_model.UserTypeOrganization,
		Visibility:                form.Visibility,
		RepoAdminChangeTeamAccess: form.RepoAdminChangeTeamAccess,
	}

	if err := organization.CreateOrganization(ctx, org, ctx.Doer); err != nil {
		ctx.Data["Err_OrgName"] = true
		switch {
		case user_model.IsErrUserAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("form.org_name_been_taken"), tplCreateOrg, &form)
		case db.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("org.form.name_reserved", err.(db.ErrNameReserved).Name), tplCreateOrg, &form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("org.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplCreateOrg, &form)
		case organization.IsErrUserNotAllowedCreateOrg(err):
			ctx.RenderWithErr(ctx.Tr("org.form.create_org_not_allowed"), tplCreateOrg, &form)
		default:
			ctx.ServerError("CreateOrganization", err)
		}
		return
	}
	log.Trace("Organization created: %s", org.Name)

	ctx.Redirect(org.AsUser().DashboardLink())
}
