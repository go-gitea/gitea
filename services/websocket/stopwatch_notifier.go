// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/pubsub"
)

type stopwatchesEvent struct {
	Type string          `json:"type"`
	Data api.StopWatches `json:"data"`
}

// PublishStopwatchesForUser fetches the user's current stopwatches and pushes
// them immediately to all connected WebSocket clients. Call this after any
// stopwatch start, stop, or cancel so that open tabs update without a reload.
func PublishStopwatchesForUser(ctx context.Context, user *user_model.User) {
	if !pubsub.DefaultBroker.HasTopicSubscribers(pubsub.UserTopic(user.ID)) {
		return
	}

	sws, err := issues_model.GetUserStopwatches(ctx, user.ID, db.ListOptions{})
	if err != nil {
		log.Error("websocket: GetUserStopwatches %d: %v", user.ID, err)
		return
	}

	data := api.StopWatches{}
	if len(sws) > 0 {
		apiSWs, err := convert.ToStopWatches(ctx, user, sws)
		if err != nil {
			if !issues_model.IsErrIssueNotExist(err) {
				log.Error("websocket: ToStopWatches: %v", err)
			}
			return
		}
		data = apiSWs
	}

	msg, err := json.Marshal(stopwatchesEvent{Type: EventStopwatches, Data: data})
	if err != nil {
		log.Error("websocket: marshal stopwatches event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(user.ID), msg)
}
