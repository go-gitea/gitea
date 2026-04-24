// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

// Wire-format event type names. These strings are the contract between the Go
// publishers and the TypeScript shared worker in web_src/js/user-events.sharedworker.ts
// — keep both sides in sync.
const (
	EventNotificationCount = "notification-count"
	EventStopwatches       = "stopwatches"
	EventLogout            = "logout"
)
