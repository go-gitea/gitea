// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

type logoutEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

// PublishLogout publishes a logout event to all WebSocket clients connected as
// the given user. sessionID identifies which session is signing out so the
// client can distinguish "this tab" from "another tab".
func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, EventLogout, logoutEvent{
		Type:      EventLogout,
		SessionID: sessionID,
	})
}
