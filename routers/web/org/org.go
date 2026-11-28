// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	// tplCreateOrg template path for create organization
	tplCreateOrg templates.TplName = "org/create"
)

// Create render the page for create organization
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_org")
	if !ctx.Doer.CanCreateOrganization() {
		ctx.ServerErrorAuto(errors.New(ctx.Locale.TrString("org.form.create_org_not_allowed")))
		return
	}

	ctx.Data["visibility"] = setting.Service.DefaultOrgVisibilityMode
	ctx.Data["repo_admin_change_team_access"] = true

	ctx.HTML(http.StatusOK, tplCreateOrg)
}

// CreatePost response for create organization
func CreatePost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.CreateOrgForm](ctx)
	if ctx.Written() {
		return
	}

	if !ctx.Doer.CanCreateOrganization() {
		ctx.ServerErrorAuto(errors.New(ctx.Locale.TrString("org.form.create_org_not_allowed")))
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
		var errNameReserved db.ErrNameReserved
		var errNamePatternNotAllowed db.ErrNamePatternNotAllowed
		switch {
		case user_model.IsErrUserAlreadyExist(err):
			ctx.JSONErrorWithField(ctx.Tr("form.org_name_been_taken"), "OrgName")
		case errors.As(err, &errNameReserved):
			ctx.JSONErrorWithField(ctx.Tr("org.form.name_reserved", errNameReserved.Name), "OrgName")
		case errors.As(err, &errNamePatternNotAllowed):
			ctx.JSONErrorWithField(ctx.Tr("org.form.name_pattern_not_allowed", errNamePatternNotAllowed.Pattern), "OrgName")
		case organization.IsErrUserNotAllowedCreateOrg(err):
			ctx.JSONError(ctx.Tr("org.form.create_org_not_allowed"))
		default:
			ctx.ServerErrorAuto(err)
		}
		return
	}
	audit.Record(ctx, audit_model.OrganizationCreate, org.AsUser())

	log.Trace("Organization created: %s", org.Name)

	ctx.JSONRedirect(org.AsUser().DashboardLink())
}
