// Copyright 2026 The Gitea Authors. All rights reserved.
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
	notify_service "code.gitea.io/gitea/services/notify"
	"code.gitea.io/gitea/services/pubsub"
)

type notificationCountEvent struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// Init starts the background goroutines that push real-time updates to
// connected WebSocket clients: notification counts and (when time-tracking
// is enabled) active stopwatches. It also registers the websocket notifier
// so that targeted pushes fire immediately when notification counts change.
func Init() error {
	notify_service.RegisterNotifier(&wsNotifier{})
	go graceful.GetManager().RunWithShutdownContext(run)
	if setting.Service.EnableTimetracking {
		go graceful.GetManager().RunWithShutdownContext(runStopwatch)
	}
	return nil
}

func publishNotificationCount(userID, count int64) {
	msg, err := json.Marshal(notificationCountEvent{
		Type:  "notification-count",
		Count: count,
	})
	if err != nil {
		log.Error("websocket: marshal notification-count event: %v", err)
		return
	}
	pubsub.DefaultBroker.Publish(pubsub.UserTopic(userID), msg)
}

func run(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: WebSocket", process.SystemProcessType, true)
	defer finished()

	if setting.UI.Notification.PushUpdateTime <= 0 {
		return
	}

	then := timeutil.TimeStampNow().Add(-2)
	timer := time.NewTicker(setting.UI.Notification.PushUpdateTime)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !pubsub.DefaultBroker.HasSubscribers() {
				then = timeutil.TimeStampNow().Add(-2)
				continue
			}

			now := timeutil.TimeStampNow().Add(-2)

			uidCounts, err := activities_model.GetUIDsAndNotificationCounts(ctx, then, now)
			if err != nil {
				log.Error("websocket: GetUIDsAndNotificationCounts: %v", err)
				continue
			}

			for _, uidCount := range uidCounts {
				publishNotificationCount(uidCount.UserID, uidCount.Count)
			}

			then = now
		}
	}
}
