// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
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
