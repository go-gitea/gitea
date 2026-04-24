// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/pubsub"
)

type stopwatchesEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func publishStopwatchesForUser(ctx context.Context, user *user_model.User, sws []*issues_model.Stopwatch) {
	var data any
	if len(sws) == 0 {
		data = []any{}
	} else {
		apiSWs, err := convert.ToStopWatches(ctx, user, sws)
		if err != nil {
			if !issues_model.IsErrIssueNotExist(err) {
				log.Error("websocket: ToStopWatches: %v", err)
			}
			return
		}
		data = apiSWs
	}

	msg, err := json.Marshal(stopwatchesEvent{Type: "stopwatches", Data: data})
	if err != nil {
		log.Error("websocket: marshal stopwatches event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(user.ID), msg)
}

// PublishStopwatchesForUser fetches the user's current stopwatches and pushes
// them immediately to all connected WebSocket clients, bypassing the periodic
// polling loop. Call this after any stopwatch start, stop, or cancel so that
// all open tabs update without waiting for the next tick.
func PublishStopwatchesForUser(ctx context.Context, user *user_model.User) {
	sws, err := issues_model.GetUserStopwatches(ctx, user.ID, db.ListOptions{})
	if err != nil {
		log.Error("websocket: GetUserStopwatches %d: %v", user.ID, err)
		return
	}
	publishStopwatchesForUser(ctx, user, sws)
}

func runStopwatch(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: WebSocket Stopwatch", process.SystemProcessType, true)
	defer finished()

	if setting.UI.Notification.PushUpdateTime <= 0 {
		return
	}

	timer := time.NewTicker(setting.UI.Notification.PushUpdateTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !pubsub.DefaultBroker.HasSubscribers() {
				continue
			}

			userStopwatches, err := issues_model.GetUIDsAndStopwatch(ctx)
			if err != nil {
				log.Error("websocket: GetUIDsAndStopwatch: %v", err)
				continue
			}
			if len(userStopwatches) == 0 {
				continue
			}

			userIDs := make([]int64, 0, len(userStopwatches))
			for _, us := range userStopwatches {
				userIDs = append(userIDs, us.UserID)
			}
			users, err := user_model.GetUserByIDs(ctx, userIDs)
			if err != nil {
				log.Error("websocket: GetUserByIDs: %v", err)
				continue
			}
			usersByID := make(map[int64]*user_model.User, len(users))
			for _, u := range users {
				usersByID[u.ID] = u
			}

			for _, us := range userStopwatches {
				u, ok := usersByID[us.UserID]
				if !ok {
					continue
				}
				publishStopwatchesForUser(ctx, u, us.StopWatches)
			}
		}
	}
}
