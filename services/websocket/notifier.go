// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	notify_service "code.gitea.io/gitea/services/notify"
)

// Init registers the websocket notifier so that real-time updates are pushed
// to connected clients on every DB write that affects their unread count,
// stopwatches, or session. The WebSocket pipeline is push-only — there is no
// periodic polling.
func Init() error {
	notify_service.RegisterNotifier(&wsNotifier{})
	return nil
}
