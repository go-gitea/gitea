// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	audit_model "gitea.dev/models/audit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/routers/web/auth"
	"gitea.dev/services/audit"
	auth_service "gitea.dev/services/auth"
	"gitea.dev/services/context"
	websocket_service "gitea.dev/services/websocket"
)

// SignOutSessions signs the doer out of all browsers and revokes their long-term authorization tokens.
func SignOutSessions(ctx *context.Context) {
	if err := user_model.RevokeUserSessions(ctx, ctx.Doer.ID); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	websocket_service.PublishLogout(ctx.Doer.ID, "")

	impersonator := audit.ImpersonatorFromContext(ctx)
	exitedImpersonated, err := auth_service.ExitImpersonatedUser(ctx.Session)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if exitedImpersonated {
		audit.RecordAs(ctx, impersonator, audit_model.UserImpersonationExit, ctx.Doer)
		if impersonator != nil && impersonator.ID != ctx.Doer.ID {
			ctx.JSONRedirect(setting.AppSubURL + "/-/admin")
			return
		}
	}

	redirectTo := auth.BuildSignOutRedirectURL(ctx)
	auth.HandleSignOut(ctx)
	ctx.JSONRedirect(redirectTo)
}
