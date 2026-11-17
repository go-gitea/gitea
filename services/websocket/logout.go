// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

const eventLogout = "logout" // keep in sync with web_src/js/types.ts

func PublishLogout(userID int64, sessionID string) {
	publishUserEvent(userID, func() []byte { return makeEventMessage(eventLogout, sessionID, nil) })
}
