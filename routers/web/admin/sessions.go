// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const tplUserSessions templates.TplName = "admin/user/sessions"

// UserSessions shows all sessions for a user
func UserSessions(ctx *context.Context) {
	u, err := user_model.GetUserByID(ctx, ctx.PathParamInt64("userid"))
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Redirect(setting.AppSubURL + "/-/admin/users")
		} else {
			ctx.ServerError("GetUserByID", err)
		}
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("%s - %s", ctx.Tr("admin.users.details"), ctx.Tr("settings.sessions"))
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["User"] = u

	sessions, err := auth_model.GetUserSessionsByUserID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("GetUserSessionsByUserID", err)
		return
	}

	ctx.Data["Sessions"] = sessions

	activeCount := 0
	for _, s := range sessions {
		if s.LogoutUnix == 0 {
			activeCount++
		}
	}
	ctx.Data["ActiveCount"] = activeCount

	ctx.HTML(http.StatusOK, tplUserSessions)
}

// RevokeUserSession revokes a single session for a user (admin action)
func RevokeUserSession(ctx *context.Context) {
	u, err := user_model.GetUserByID(ctx, ctx.PathParamInt64("userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	form := web.GetForm(ctx).(*forms.RevokeSessionForm)
	if form.SessionID == "" {
		ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
		ctx.Redirect(fmt.Sprintf("%s/-/admin/users/%d/sessions", setting.AppSubURL, u.ID))
		return
	}

	// Verify the session belongs to the target user
	sess, err := auth_model.GetUserSessionByID(ctx, form.SessionID)
	if err != nil {
		if auth_model.IsErrUserSessionNotExist(err) {
			ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
			ctx.Redirect(fmt.Sprintf("%s/-/admin/users/%d/sessions", setting.AppSubURL, u.ID))
			return
		}
		ctx.ServerError("GetUserSessionByID", err)
		return
	}
	if sess.UserID != u.ID {
		ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
		ctx.Redirect(fmt.Sprintf("%s/-/admin/users/%d/sessions", setting.AppSubURL, u.ID))
		return
	}

	if err := auth_model.InvalidateUserSession(ctx, form.SessionID); err != nil {
		ctx.ServerError("InvalidateUserSession", err)
		return
	}

	if err := session.DestroySessionByID(form.SessionID); err != nil {
		log.Error("Failed to destroy chi-session %s: %v", form.SessionID, err)
	}

	// Delete the specific remember-me auth token so the browser can't auto-sign back in
	if sess.AuthTokenID != "" {
		if err := auth_model.DeleteAuthTokenByID(ctx, sess.AuthTokenID); err != nil {
			log.Error("Failed to delete auth token %s: %v", sess.AuthTokenID, err)
		}
	}

	ctx.Flash.Success(ctx.Tr("settings.sessions.revoke_success"))
	ctx.Redirect(fmt.Sprintf("%s/-/admin/users/%d/sessions", setting.AppSubURL, u.ID))
}

// RevokeAllUserSessions revokes all sessions for a user (admin action)
func RevokeAllUserSessions(ctx *context.Context) {
	u, err := user_model.GetUserByID(ctx, ctx.PathParamInt64("userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	sessions, err := auth_model.GetUserSessionsByUserID(ctx, u.ID)
	if err != nil {
		ctx.ServerError("GetUserSessionsByUserID", err)
		return
	}

	// Destroy active chi-sessions first, then bulk-update DB metadata.
	// DestroySessionByID is what actually revokes the session; InvalidateAllUserSessions
	// only sets logout_unix for audit purposes (there is no per-request DB validity check).
	for _, s := range sessions {
		if s.LogoutUnix == 0 {
			if err := session.DestroySessionByID(s.ID); err != nil {
				log.Error("Failed to destroy chi-session %s: %v", s.ID, err)
			}
		}
	}

	if err := auth_model.InvalidateAllUserSessions(ctx, u.ID, ""); err != nil {
		ctx.ServerError("InvalidateAllUserSessions", err)
		return
	}

	// Delete remember-me auth tokens so revoked sessions can't be restored
	if err := auth_model.DeleteAuthTokensByUserID(ctx, u.ID); err != nil {
		log.Error("Failed to delete auth tokens for user %d: %v", u.ID, err)
	}

	ctx.Flash.Success(ctx.Tr("settings.sessions.revoke_all_success"))
	ctx.Redirect(fmt.Sprintf("%s/-/admin/users/%d/sessions", setting.AppSubURL, u.ID))
}
