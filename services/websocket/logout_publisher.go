// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/pubsub"
)

type logoutEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

// PublishLogout publishes a logout event to all WebSocket clients connected as
// the given user. sessionID identifies which session is signing out so the
// client can distinguish "this tab" from "another tab".
func PublishLogout(userID int64, sessionID string) {
	msg, err := json.Marshal(logoutEvent{
		Type:      "logout",
		SessionID: sessionID,
	})
	if err != nil {
		log.Error("websocket: marshal logout event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}
