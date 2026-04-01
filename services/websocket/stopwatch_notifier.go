// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/pubsub"
)

type stopwatchesEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// PublishEmptyStopwatches immediately pushes an empty stopwatches list to the
// given user's WebSocket clients — used when the user's last stopwatch is cancelled.
func PublishEmptyStopwatches(userID int64) {
	msg, err := json.Marshal(stopwatchesEvent{
		Type: "stopwatches",
		Data: json.RawMessage(`[]`),
	})
	if err != nil {
		log.Error("websocket: marshal empty stopwatches: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}

// InitStopwatch starts the background goroutine that polls active stopwatches
// and pushes updates to connected WebSocket clients.
func InitStopwatch() error {
	if !setting.Service.EnableTimetracking {
		return nil
	}
	go graceful.GetManager().RunWithShutdownContext(runStopwatch)
	return nil
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

				dataBs, err := json.Marshal(apiSWs)
				if err != nil {
					log.Error("websocket: marshal stopwatches: %v", err)
					continue
				}

				msg, err := json.Marshal(stopwatchesEvent{
					Type: "stopwatches",
					Data: dataBs,
				})
				if err != nil {
					continue
				}
				pubsub.DefaultBroker.Publish(pubsub.UserTopic(us.UserID), msg)
			}
		}
	}
}
