// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

const tplSettingsSecuritySessions templates.TplName = "user/settings/security/sessions"

// Sessions renders the user's active sessions page
func Sessions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.sessions")
	ctx.Data["PageIsSettingsSecurity"] = true

	sessions, err := auth_model.GetUserSessionsByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserSessionsByUserID", err)
		return
	}

	ctx.Data["Sessions"] = sessions
	ctx.Data["CurrentSessionID"] = ctx.Session.ID()

	otherActive := 0
	for _, s := range sessions {
		if s.LogoutUnix == 0 && s.ID != ctx.Session.ID() {
			otherActive++
		}
	}
	ctx.Data["OtherActiveCount"] = otherActive

	ctx.HTML(http.StatusOK, tplSettingsSecuritySessions)
}

// RevokeSession revokes a single user session
func RevokeSession(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RevokeSessionForm)
	if form.SessionID == "" {
		ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/security/sessions")
		return
	}

	// Verify the session belongs to the current user
	sess, err := auth_model.GetUserSessionByID(ctx, form.SessionID)
	if err != nil {
		if auth_model.IsErrUserSessionNotExist(err) {
			ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/security/sessions")
			return
		}
		ctx.ServerError("GetUserSessionByID", err)
		return
	}
	if sess.UserID != ctx.Doer.ID {
		ctx.Flash.Error(ctx.Tr("settings.sessions.session_not_found"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/security/sessions")
		return
	}

	// Mark as logged out
	if err := auth_model.InvalidateUserSession(ctx, form.SessionID); err != nil {
		ctx.ServerError("InvalidateUserSession", err)
		return
	}

	// Destroy the chi-session record via the provider
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
	ctx.Redirect(setting.AppSubURL + "/user/settings/security/sessions")
}

// RevokeAllSessions revokes all sessions except the current one
func RevokeAllSessions(ctx *context.Context) {
	currentSessionID := ctx.Session.ID()

	// Get all active sessions for the user to destroy their chi-sessions
	sessions, err := auth_model.GetUserSessionsByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserSessionsByUserID", err)
		return
	}

	// Destroy active chi-sessions first, then bulk-update DB metadata.
	// DestroySessionByID is what actually revokes the session; InvalidateAllUserSessions
	// only sets logout_unix for audit purposes (there is no per-request DB validity check).
	for _, s := range sessions {
		if s.ID != currentSessionID && s.LogoutUnix == 0 {
			if err := session.DestroySessionByID(s.ID); err != nil {
				log.Error("Failed to destroy chi-session %s: %v", s.ID, err)
			}
		}
	}

	// Invalidate all sessions except current
	if err := auth_model.InvalidateAllUserSessions(ctx, ctx.Doer.ID, currentSessionID); err != nil {
		ctx.ServerError("InvalidateAllUserSessions", err)
		return
	}

	// Delete remember-me auth tokens so revoked sessions can't be restored
	if err := auth_model.DeleteAuthTokensByUserID(ctx, ctx.Doer.ID); err != nil {
		log.Error("Failed to delete auth tokens for user %d: %v", ctx.Doer.ID, err)
	}

	ctx.Flash.Success(ctx.Tr("settings.sessions.revoke_all_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/security/sessions")
}
