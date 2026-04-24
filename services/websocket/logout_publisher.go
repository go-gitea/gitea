// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

type logoutEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

// sessionID identifies the session that is signing out so connected tabs can
// distinguish the originating session from others.
func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, logoutEvent{
		Type:      EventLogout,
		SessionID: sessionID,
	})
}
