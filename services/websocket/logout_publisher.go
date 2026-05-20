// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

// LogoutBrokerMsg is the bus-side wire format; the WS handler rewrites SessionID
// into "here"/"elsewhere" per connection. Empty SessionID targets all sessions.
type LogoutBrokerMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, LogoutBrokerMsg{
		Type:      EventLogout,
		SessionID: sessionID,
	})
}
