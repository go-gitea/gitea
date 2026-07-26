// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

// PublishLogout logs out one session of the user, or all of them when sessionID is empty.
func PublishLogout(userID int64, sessionID string) {
	publishSessionEvent(userID, sessionID, EventLogout, nil)
}
