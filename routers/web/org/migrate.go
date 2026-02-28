// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	gocontext "context"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/migrations"
)

const tplOrgMigrate templates.TplName = "org/migrate"

// MigrateOrg render organization migration page
func MigrateOrg(ctx *context.Context) {
	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "MigrateOrg: the site administrator has disabled migrations")
		return
	}

	ctx.Data["Title"] = ctx.Tr("org.migrate.title")
	ctx.Data["LFSActive"] = setting.LFS.StartServer
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull

	// Plain git should be first
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	ctx.Data["service"] = structs.GitServiceType(ctx.FormInt("service_type"))

	// Get organizations the user can migrate to
	orgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return
	}
	ctx.Data["Orgs"] = orgs

	ctx.HTML(http.StatusOK, tplOrgMigrate)
}

// MigrateOrgPost handles organization migration form submission
func MigrateOrgPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MigrateOrgForm)

	if setting.Repository.DisableMigrations {
		ctx.HTTPError(http.StatusForbidden, "MigrateOrgPost: the site administrator has disabled migrations")
		return
	}

	if form.Mirror && setting.Mirror.DisableNewPull {
		ctx.HTTPError(http.StatusBadRequest, "MigrateOrgPost: the site administrator has disabled creation of new mirrors")
		return
	}

	ctx.Data["Title"] = ctx.Tr("org.migrate.title")
	ctx.Data["LFSActive"] = setting.LFS.StartServer
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	ctx.Data["service"] = form.Service

	// Get organizations the user can migrate to
	orgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return
	}
	ctx.Data["Orgs"] = orgs

	// Validate target organization
	targetOrg, err := organization.GetOrgByName(ctx, form.TargetOrgName)
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			ctx.Data["Err_TargetOrgName"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("org.migrate.target_org_not_exist"), tplOrgMigrate, form)
			return
		}
		ctx.ServerError("GetOrgByName", err)
		return
	}

	// Check permissions - user must be an owner of the target organization
	isOwner, err := targetOrg.IsOwnedBy(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("IsOwnedBy", err)
		return
	}
	if !isOwner && !ctx.Doer.IsAdmin {
		ctx.Data["Err_TargetOrgName"] = true
		ctx.RenderWithErrDeprecated(ctx.Tr("org.migrate.permission_denied"), tplOrgMigrate, form)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplOrgMigrate)
		return
	}

	// Validate source URL
	remoteAddr := form.CloneAddr
	err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.Doer)
	if err != nil {
		ctx.Data["Err_CloneAddr"] = true
		ctx.RenderWithErrDeprecated(ctx.Tr("form.url_error", remoteAddr), tplOrgMigrate, form)
		return
	}

	opts := migrations.OrgMigrateOptions{
		CloneAddr:      form.CloneAddr,
		AuthUsername:   form.AuthUsername,
		AuthPassword:   form.AuthPassword,
		AuthToken:      form.AuthToken,
		TargetOrgName:  form.TargetOrgName,
		SourceOrgName:  form.SourceOrgName,
		GitServiceType: form.Service,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror,
		LFS:            form.LFS && setting.LFS.StartServer,
		LFSEndpoint:    form.LFSEndpoint,
		Wiki:           form.Wiki,
		Issues:         form.Issues,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Releases:       form.Releases,
		ReleaseAssets:  form.ReleaseAssets,
		Comments:       form.Issues || form.PullRequests,
		PullRequests:   form.PullRequests,
		MirrorInterval: form.MirrorInterval,
	}

	// Perform the migration in background context
	doLongTimeMigrate := func(ctx gocontext.Context, doer *user_model.User) (*migrations.OrgMigrationResult, error) {
		return migrations.MigrateOrganization(ctx, doer, opts, nil)
	}

	// Use hammer context to not cancel if client disconnects
	result, err := doLongTimeMigrate(graceful.GetManager().HammerContext(), ctx.Doer)
	if err != nil {
		if migrations.IsRateLimitError(err) {
			ctx.RenderWithErrDeprecated(ctx.Tr("form.visit_rate_limit"), tplOrgMigrate, form)
		} else if base.IsErrNotSupported(err) {
			ctx.Data["Err_CloneAddr"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("org.migrate.service_not_supported"), tplOrgMigrate, form)
		} else {
			log.Error("Failed to migrate organization: %v", err)
			ctx.RenderWithErrDeprecated(ctx.Tr("org.migrate.failed", err.Error()), tplOrgMigrate, form)
		}
		return
	}

	// Show success page with results
	ctx.Data["Result"] = result
	ctx.Data["TargetOrgName"] = form.TargetOrgName
	ctx.HTML(http.StatusOK, templates.TplName("org/migrate_success"))
}
