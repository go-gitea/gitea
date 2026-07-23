// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	gocontext "context"
	"errors"
	"net/http"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const tplCodespaceGovernance templates.TplName = "codespace/governance"

// AdminList renders the site Codespace governance list.
func AdminList(ctx *context.Context) {
	ctx.Data["PageIsAdminCodespaces"] = true
	renderGovernanceList(ctx, codespace_service.GovernanceScopeSite, 0, setting.AppSubURL+"/-/admin/codespaces")
}

func renderGovernanceList(ctx *context.Context, scope string, ownerID int64, actionBase string) {
	result, err := codespace_service.ListGovernanceCodespaces(ctx, codespace_service.GovernanceListOptions{
		Scope:   scope,
		OwnerID: ownerID,
	})
	if err != nil {
		ctx.ServerError("ListGovernanceCodespaces", err)
		return
	}
	ctx.Data["Title"] = "Codespaces"
	ctx.Data["Codespaces"] = result.Rows
	ctx.Data["ActionBase"] = actionBase
	ctx.Data["IsSiteGovernance"] = scope == codespace_service.GovernanceScopeSite
	ctx.HTML(http.StatusOK, tplCodespaceGovernance)
}

// AdminStop queues a stop operation from the site governance list.
func AdminStop(ctx *context.Context) {
	governanceAction(ctx, codespace_service.GovernanceScopeSite, 0, codespace_service.StopGovernanceCodespace, setting.AppSubURL+"/-/admin/codespaces")
}

// AdminDelete deletes or queues deletion from the site governance list.
func AdminDelete(ctx *context.Context) {
	governanceAction(ctx, codespace_service.GovernanceScopeSite, 0, codespace_service.DeleteGovernanceCodespace, setting.AppSubURL+"/-/admin/codespaces")
}

// AdminForceDelete physically deletes one Codespace from the site governance list.
func AdminForceDelete(ctx *context.Context) {
	if ctx.FormString("confirm") != "force-delete" {
		ctx.PlainText(http.StatusBadRequest, "confirm_required")
		return
	}
	err := codespace_service.ForceDeleteCodespace(ctx, codespace_service.GovernanceActionOptions{
		Scope:         codespace_service.GovernanceScopeSite,
		CodespaceUUID: ctx.PathParam("uuid"),
	})
	if err != nil {
		handleGovernanceActionError(ctx, "ForceDeleteCodespace", err)
		return
	}
	ctx.Redirect(setting.AppSubURL+"/-/admin/codespaces", http.StatusSeeOther)
}

// OrgStop queues a stop operation from the organization governance list.
func OrgStop(ctx *context.Context) {
	if ctx.Org == nil || ctx.Org.Organization == nil {
		ctx.NotFound(nil)
		return
	}
	governanceAction(ctx, codespace_service.GovernanceScopeOrganization, ctx.Org.Organization.ID, codespace_service.StopGovernanceCodespace, ctx.Org.OrgLink+"/settings/codespaces")
}

// OrgDelete deletes or queues deletion from the organization governance list.
func OrgDelete(ctx *context.Context) {
	if ctx.Org == nil || ctx.Org.Organization == nil {
		ctx.NotFound(nil)
		return
	}
	governanceAction(ctx, codespace_service.GovernanceScopeOrganization, ctx.Org.Organization.ID, codespace_service.DeleteGovernanceCodespace, ctx.Org.OrgLink+"/settings/codespaces")
}

type governanceActionFunc func(gocontext.Context, codespace_service.GovernanceActionOptions) (*codespace_service.LifecycleActionResult, error)

func governanceAction(ctx *context.Context, scope string, ownerID int64, fn governanceActionFunc, redirectTo string) {
	_, err := fn(ctx, codespace_service.GovernanceActionOptions{
		Scope:         scope,
		OwnerID:       ownerID,
		CodespaceUUID: ctx.PathParam("uuid"),
	})
	if err != nil {
		handleGovernanceActionError(ctx, "GovernanceCodespaceAction", err)
		return
	}
	ctx.Redirect(redirectTo, http.StatusSeeOther)
}

func handleGovernanceActionError(ctx *context.Context, name string, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrGovernanceNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrGovernanceStateUnavailable):
		ctx.PlainText(http.StatusConflict, "state_unavailable")
	case errors.Is(err, codespace_service.ErrLifecycleActionVersionExhausted):
		ctx.PlainText(http.StatusConflict, "version_exhausted")
	default:
		ctx.ServerError(name, err)
	}
}
