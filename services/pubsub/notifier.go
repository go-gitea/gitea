// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

func InitWithNotifier() Broker {
	broker := NewMemory() // TODO: allow for other pubsub implementations
	notify_service.RegisterNotifier(newNotifier(broker))
	return broker
}

type pubsubNotifier struct {
	notify_service.NullNotifier
	broker Broker
}

// NewNotifier create a new pubsubNotifier notifier
func newNotifier(broker Broker) notify_service.Notifier {
	return &pubsubNotifier{
		broker: broker,
	}
}

func (p *pubsubNotifier) DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	data := struct {
		Function string
		Comment  *issues_model.Comment
		Doer     *user_model.User
	}{
		Function: "DeleteComment",
		Comment:  c,
		Doer:     doer,
	}

	msg, err := json.Marshal(data)
	if err != nil {
		log.Error("Failed to marshal message: %v", err)
		return
	}

	err = p.broker.Publish(ctx, "notify", msg)
	if err != nil {
		log.Error("Failed to publish message: %v", err)
		return
	}
}
