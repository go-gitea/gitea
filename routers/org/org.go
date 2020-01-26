// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"errors"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplCreateOrg template path for create organization
	tplCreateOrg base.TplName = "org/create"
)

// Create render the page for create organization
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_org")
	ctx.Data["DefaultOrgVisibilityMode"] = setting.Service.DefaultOrgVisibilityMode
	if !ctx.User.CanCreateOrganization() {
		ctx.ServerError("Not allowed", errors.New(ctx.Tr("org.form.create_org_not_allowed")))
		return
	}
	ctx.HTML(200, tplCreateOrg)
}

// CreatePost response for create organization
func CreatePost(ctx *context.Context, form auth.CreateOrgForm) {
	ctx.Data["Title"] = ctx.Tr("new_org")

	if !ctx.User.CanCreateOrganization() {
		ctx.ServerError("Not allowed", errors.New(ctx.Tr("org.form.create_org_not_allowed")))
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, tplCreateOrg)
		return
	}

	org := &models.User{
		Name:                      form.OrgName,
		IsActive:                  true,
		Type:                      models.UserTypeOrganization,
		Visibility:                form.Visibility,
		RepoAdminChangeTeamAccess: form.RepoAdminChangeTeamAccess,
	}

	if err := models.CreateOrganization(org, ctx.User); err != nil {
		ctx.Data["Err_OrgName"] = true
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("form.org_name_been_taken"), tplCreateOrg, &form)
		case models.IsErrNameReserved(err):
			ctx.RenderWithErr(ctx.Tr("org.form.name_reserved", err.(models.ErrNameReserved).Name), tplCreateOrg, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.RenderWithErr(ctx.Tr("org.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplCreateOrg, &form)
		case models.IsErrUserNotAllowedCreateOrg(err):
			ctx.RenderWithErr(ctx.Tr("org.form.create_org_not_allowed"), tplCreateOrg, &form)
		default:
			ctx.ServerError("CreateOrganization", err)
		}
		return
	}
	log.Trace("Organization created: %s", org.Name)

	ctx.Redirect(setting.AppSubURL + "/org/" + form.OrgName + "/dashboard")
}
