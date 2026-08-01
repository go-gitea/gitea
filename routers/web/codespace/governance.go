// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	gocontext "context"
	"errors"
	"net/http"
	"strconv"

	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

// AdminStop queues a stop operation from a Manager or unassigned Codespace list.
func AdminStop(ctx *context.Context) {
	governanceAction(ctx, codespace_service.StopGovernanceCodespace)
}

// AdminDelete deletes or queues deletion from a Manager or unassigned Codespace list.
func AdminDelete(ctx *context.Context) {
	governanceAction(ctx, codespace_service.DeleteGovernanceCodespace)
}

// AdminForceDelete physically deletes one Codespace from a Manager or unassigned Codespace list.
func AdminForceDelete(ctx *context.Context) {
	opts, redirectTo := governanceTarget(ctx)
	if ctx.FormString("confirm") != "force-delete" {
		ctx.Flash.Error(ctx.Tr("codespace.error.confirm_required"))
		ctx.Redirect(redirectTo, http.StatusSeeOther)
		return
	}
	err := codespace_service.ForceDeleteCodespace(ctx, opts)
	if err != nil {
		handleGovernanceActionError(ctx, "ForceDeleteCodespace", redirectTo, err)
		return
	}
	ctx.Redirect(redirectTo, http.StatusSeeOther)
}

type governanceActionFunc func(gocontext.Context, codespace_service.GovernanceActionOptions) (*codespace_service.LifecycleActionResult, error)

func governanceAction(ctx *context.Context, fn governanceActionFunc) {
	opts, redirectTo := governanceTarget(ctx)
	_, err := fn(ctx, opts)
	if err != nil {
		handleGovernanceActionError(ctx, "GovernanceCodespaceAction", redirectTo, err)
		return
	}
	ctx.Redirect(redirectTo, http.StatusSeeOther)
}

func governanceTarget(ctx *context.Context) (codespace_service.GovernanceActionOptions, string) {
	managerID := ctx.PathParamInt64("manager_id")
	base := setting.AppSubURL + "/-/admin/codespaces/managers"
	if managerID > 0 {
		return codespace_service.GovernanceActionOptions{
			CodespaceUUID: ctx.PathParam("uuid"),
			ManagerID:     managerID,
		}, base + "/" + strconv.FormatInt(managerID, 10)
	}
	return codespace_service.GovernanceActionOptions{
		CodespaceUUID: ctx.PathParam("uuid"),
		Unassigned:    true,
	}, base
}

func handleGovernanceActionError(ctx *context.Context, name, redirectTo string, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrGovernanceNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrGovernanceStateUnavailable):
		ctx.Flash.Warning(ctx.Tr("codespace.error.state_unavailable"))
		ctx.Redirect(redirectTo, http.StatusSeeOther)
	case errors.Is(err, codespace_service.ErrLifecycleActionVersionExhausted):
		ctx.Flash.Warning(ctx.Tr("codespace.error.version_exhausted"))
		ctx.Redirect(redirectTo, http.StatusSeeOther)
	default:
		ctx.ServerError(name, err)
	}
}
