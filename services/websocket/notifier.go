// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/pubsub"
)

// nowTS returns the current time as a TimeStamp using the real wall clock,
// avoiding data races with timeutil.MockUnset during tests.
func nowTS() timeutil.TimeStamp {
	return timeutil.TimeStamp(time.Now().Unix())
}

type notificationCountEvent struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// Init starts the background goroutine that polls notification counts
// and pushes updates to connected WebSocket clients.
func Init() error {
	go graceful.GetManager().RunWithShutdownContext(run)
	return nil
}

func run(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: WebSocket", process.SystemProcessType, true)
	defer finished()

	if setting.UI.Notification.EventSourceUpdateTime <= 0 {
		return
	}

	then := nowTS().Add(-2)
	timer := time.NewTicker(setting.UI.Notification.EventSourceUpdateTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !pubsub.DefaultBroker.HasSubscribers() {
				then = nowTS().Add(-2)
				continue
			}

			now := nowTS().Add(-2)

			uidCounts, err := activities_model.GetUIDsAndNotificationCounts(ctx, then, now)
			if err != nil {
				log.Error("websocket: GetUIDsAndNotificationCounts: %v", err)
				continue
			}

			for _, uidCount := range uidCounts {
				msg, err := json.Marshal(notificationCountEvent{
					Type:  "notification-count",
					Count: uidCount.Count,
				})
				if err != nil {
					continue
				}
				pubsub.DefaultBroker.Publish(pubsub.UserTopic(uidCount.UserID), msg)
			}

			then = now
		}
	}
}
