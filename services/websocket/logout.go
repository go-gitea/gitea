// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

const eventLogout = "logout" // keep in sync with web_src/js/types.ts

// PublishLogout signs out one session, or all when sessionID is empty.
func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, func() []byte { return makeSessionEventMessage(eventLogout, sessionID, nil) })
}
