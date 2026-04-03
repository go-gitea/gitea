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
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(user.ID), msg)
}

func runStopwatch(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: WebSocket Stopwatch", process.SystemProcessType, true)
	defer finished()

	if setting.UI.Notification.EventSourceUpdateTime <= 0 {
		return
	}

	timer := time.NewTicker(setting.UI.Notification.EventSourceUpdateTime)
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

			for _, us := range userStopwatches {
				u, err := user_model.GetUserByID(ctx, us.UserID)
				if err != nil {
					log.Error("websocket: GetUserByID %d: %v", us.UserID, err)
					continue
				}

				apiSWs, err := convert.ToStopWatches(ctx, u, us.StopWatches)
				if err != nil {
					if !issues_model.IsErrIssueNotExist(err) {
						log.Error("websocket: ToStopWatches: %v", err)
					}
					continue
				}

				msg, err := json.Marshal(stopwatchesEvent{
					Type: "stopwatches",
					Data: apiSWs,
				})
				if err != nil {
					continue
				}
				pubsub.DefaultBroker.Publish(pubsub.UserTopic(us.UserID), msg)
			}
		}
	}
}
