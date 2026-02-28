// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	gocontext "context"
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/migrations"
)

// MigrateOrg migrates all repositories from an organization on another platform
func MigrateOrg(ctx *context.APIContext) {
	// swagger:operation POST /orgs/migrate organization orgMigrate
	// ---
	// summary: Migrate an organization's repositories
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MigrateOrgOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/OrgMigrationResult"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MigrateOrgOptions)

	// Check if migrations are enabled
	if setting.Repository.DisableMigrations {
		ctx.APIError(http.StatusForbidden, errors.New("the site administrator has disabled migrations"))
		return
	}

	// Get target organization
	targetOrg, err := organization.GetOrgByName(ctx, form.TargetOrgName)
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("target organization '%s' does not exist", form.TargetOrgName))
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Check permissions - user must be an owner of the target organization
	if !ctx.Doer.IsAdmin {
		isOwner, err := targetOrg.IsOwnedBy(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if !isOwner {
			ctx.APIError(http.StatusForbidden, "Only organization owners can migrate repositories")
			return
		}
	}

	// Validate the source URL before attempting migration
	if err := migrations.IsMigrateURLAllowed(form.CloneAddr, ctx.Doer); err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err)
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
		Private:        form.Private,
		Mirror:         form.Mirror,
		LFS:            form.LFS,
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
		result, err := migrations.MigrateOrganization(ctx, doer, opts, nil)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Use hammer context to not cancel if client disconnects
	result, err := doLongTimeMigrate(graceful.GetManager().HammerContext(), ctx.Doer)
	if err != nil {
		if migrations.IsRateLimitError(err) {
			ctx.APIError(http.StatusUnprocessableEntity, "Remote visit addressed rate limitation.")
		} else if base.IsErrNotSupported(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			log.Error("Failed to migrate organization: %v", err)
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Convert result to API response
	apiResult := &api.OrgMigrationResult{
		TotalRepos:    result.TotalRepos,
		MigratedRepos: result.MigratedRepos,
		FailedRepos:   make([]api.OrgMigrationFailure, len(result.FailedRepos)),
	}

	for i, f := range result.FailedRepos {
		apiResult.FailedRepos[i] = api.OrgMigrationFailure{
			RepoName: f.RepoName,
			Error:    f.Error.Error(),
		}
	}

	ctx.JSON(http.StatusCreated, apiResult)
}
