// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/services/convert"
	"gitea.dev/services/pubsub"
)

func (n *wsNotifier) StopwatchChanged(ctx context.Context, user *user_model.User) {
	if !pubsub.DefaultBroker.HasTopicSubscribers(pubsub.UserTopic(user.ID)) {
		return
	}

	sws, err := issues_model.GetUserStopwatches(ctx, user.ID, db.ListOptions{})
	if err != nil {
		log.Error("websocket: GetUserStopwatches %d: %v", user.ID, err)
		return
	}

	apiStopWatches, err := convert.ToStopWatches(ctx, user, sws)
	if err != nil {
		if !issues_model.IsErrIssueNotExist(err) {
			log.Error("websocket: ToStopWatches: %v", err)
		}
		return
	}
	publishUserEvent(user.ID, EventStopwatches, util.SliceNilAsEmpty(apiStopWatches))
}
