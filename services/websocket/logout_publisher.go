// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

type LogoutEventData struct {
	SessionID string `json:"sessionID,omitempty"`
}

func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, EventLogout, LogoutEventData{SessionID: sessionID})
}
