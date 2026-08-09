// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"
	"gitea.dev/services/pubsub"
)

func Init() error {
	// the pubsub broker must be ready before the notifier starts publishing to it
	if err := pubsub.Init(); err != nil {
		return err
	}
	notify_service.RegisterNotifier(&wsNotifier{})
	return nil
}

func SubscribeUser(user *user_model.User) (<-chan []byte, func()) {
	return pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(user.ID))
}

// ConnectSnapshot returns the frames that open the stream, so a client never reconciles over a separate request.
func ConnectSnapshot(ctx context.Context, user *user_model.User) (frames [][]byte) {
	if sws, ok := userStopwatches(ctx, user); ok {
		frames = append(frames, MakeUserEventMessage(EventStopwatches, sws))
	}
	if count, ok := unreadNotificationCount(ctx, user.ID); ok {
		frames = append(frames, MakeUserEventMessage(EventNotificationCount, count))
	}
	return frames
}
