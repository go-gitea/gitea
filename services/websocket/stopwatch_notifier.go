// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/pubsub"
)

type stopwatchesEvent struct {
	Type string          `json:"type"`
	Data api.StopWatches `json:"data"`
}

// Call after any stopwatch start/stop/cancel so connected tabs refresh.
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

	publishUserEvent(user.ID, stopwatchesEvent{Type: EventStopwatches, Data: data})
}
