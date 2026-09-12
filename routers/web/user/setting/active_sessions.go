// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/log"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const tplActiveSessions templates.TplName = "user/settings/active_sessions"

// ActiveSessions lists all active sessions for the current user.
func ActiveSessions(ctx *context.Context) {
	if !setting.SessionConfig.EnableSessionManager {
		ctx.NotFound(errors.New("session manager is disabled"))
		return
	}

	ctx.Data["Title"] = ctx.Tr("settings.active_sessions")
	ctx.Data["PageIsSettingsActiveSessions"] = true

	stores, err := ctx.Session.ListStoreByIndexer(ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("ListStoreByIndexer", err)
		return
	}

	type sessionInfo struct {
		ID           string
		IP           string
		SignInTime   time.Time
		SignInMethod string
		UserAgent    string
		IsCurrent    bool
		IsMobile     bool
	}

	currentSID := ctx.Session.ID()
	sessions := make([]sessionInfo, 0, len(stores))

	for _, s := range stores {
		ip, _ := s.Get(session.KeySignInIP).(string)
		signInUnix, _ := s.Get(session.KeySignInTime).(int64)
		ua, _ := s.Get(session.KeySignInUserAgent).(string)
		method, _ := s.Get(session.KeySignInMethod).(string)

		var signInTime time.Time
		if signInUnix > 0 {
			signInTime = time.Unix(signInUnix, 0)
		}

		sessions = append(sessions, sessionInfo{
			ID:           s.ID(),
			IP:           stripPort(ip),
			SignInTime:   signInTime,
			SignInMethod: string(ctx.Tr(signInMethodLabel(method))),
			UserAgent:    summarizeUserAgent(ua),
			IsCurrent:    s.ID() == currentSID,
			IsMobile:     isMobileUA(ua),
		})
	}

	ctx.Data["Sessions"] = sessions
	ctx.Data["SessionCount"] = len(sessions)
	ctx.HTML(http.StatusOK, tplActiveSessions)
}

// RevokeSession handles POST to revoke a specific session.
func RevokeSession(ctx *context.Context) {
	if !setting.SessionConfig.EnableSessionManager {
		ctx.NotFound(errors.New("session manager is disabled"))
		return
	}

	sid := ctx.FormString("session_id")
	if sid == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if sid == ctx.Session.ID() {
		ctx.Flash.Error(ctx.Tr("settings.active_sessions.cannot_revoke_current"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/active_sessions")
		return
	}

	// If the session was created via remember-me, also delete the auth token.
	deleteRememberToken(ctx, sid)

	if err := ctx.Session.DestroySessionByID(sid, ctx.Doer.ID); err != nil {
		ctx.ServerError("DestroySessionByID", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.active_sessions.revoke_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/active_sessions")
}

// deleteRememberToken looks up the target session and deletes its associated
// remember-me auth token if one exists. Failure is logged but not fatal.
func deleteRememberToken(ctx *context.Context, sid string) {
	stores, err := ctx.Session.ListStoreByIndexer(ctx.Doer.ID)
	if err != nil {
		return
	}
	for _, s := range stores {
		if s.ID() == sid {
			if tokenID, ok := s.Get(session.KeyRememberTokenID).(string); ok && tokenID != "" {
				if err := auth_model.DeleteAuthTokenByID(ctx, tokenID); err != nil {
					log.Error("Failed to delete auth token %s during session revoke: %v", tokenID, err)
				}
			}
			break
		}
	}
}

// signInMethodLabel maps the internal sign-in method key to an i18n key.
func signInMethodLabel(method string) string {
	switch method {
	case session.SignInMethodPassword:
		return "settings.active_sessions.signin_method.password"
	case session.SignInMethodOAuth2:
		return "settings.active_sessions.signin_method.oauth2"
	case session.SignInMethodRemember:
		return "settings.active_sessions.signin_method.remember"
	default:
		return ""
	}
}

// summarizeUserAgent extracts a human-readable "Browser on OS" summary from a raw User-Agent header.
func summarizeUserAgent(ua string) string {
	if ua == "" {
		return ""
	}

	// Firefox
	if strings.Contains(ua, "Firefox/") {
		browser := "Firefox"
		if idx := strings.Index(ua, "Firefox/"); idx >= 0 {
			if end := strings.IndexByte(ua[idx:], ' '); end > 0 {
				browser = ua[idx : idx+end]
			}
		}
		os := detectOS(ua)
		if os != "" {
			return browser + " on " + os
		}
		return browser
	}

	// Chrome (check before Safari to avoid false match)
	if strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Edg/") {
		browser := "Chrome"
		if idx := strings.Index(ua, "Chrome/"); idx >= 0 {
			if end := strings.IndexByte(ua[idx:], ' '); end > 0 {
				browser = ua[idx : idx+end]
			}
		}
		os := detectOS(ua)
		if os != "" {
			return browser + " on " + os
		}
		return browser
	}

	// Safari
	if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/") {
		browser := "Safari"
		if idx := strings.Index(ua, "Version/"); idx >= 0 {
			if end := strings.IndexByte(ua[idx:], ' '); end > 0 {
				browser = "Safari " + ua[idx+8:idx+end]
			}
		}
		os := detectOS(ua)
		if os != "" {
			return browser + " on " + os
		}
		return browser
	}

	// Edge
	if strings.Contains(ua, "Edg/") {
		browser := "Edge"
		if idx := strings.Index(ua, "Edg/"); idx >= 0 {
			if end := strings.IndexByte(ua[idx:], ' '); end > 0 {
				browser = ua[idx : idx+end]
			}
		}
		os := detectOS(ua)
		if os != "" {
			return browser + " on " + os
		}
		return browser
	}

	return ua
}

// isMobileUA detects whether a raw User-Agent string belongs to a mobile device.
func isMobileUA(ua string) bool {
	return strings.Contains(ua, "Mobile") ||
		strings.Contains(ua, "Android") && !strings.Contains(ua, "Tablet") ||
		strings.Contains(ua, "iPhone") ||
		strings.Contains(ua, "iPad")
}

// stripPort removes the port number from an address string (e.g. "192.168.8.246:55258" → "192.168.8.246").
// If the address is not in host:port format, it is returned unchanged.
func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// detectOS extracts a human-readable OS name from a User-Agent string.
func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT 10"):
		return "Windows 10/11"
	case strings.Contains(ua, "Windows NT 6.3"):
		return "Windows 8.1"
	case strings.Contains(ua, "Windows NT 6.1"):
		return "Windows 7"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "macOS"):
		return "macOS"
	case strings.Contains(ua, "X11; Linux"):
		return "Linux"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		return "iOS"
	default:
		return ""
	}
}
