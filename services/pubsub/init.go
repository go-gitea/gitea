// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"fmt"

	"gitea.dev/modules/setting"
)

// Init replaces DefaultBroker according to setting.Websocket. Called from
// websocket.Init before the notifier is registered so subscribers wire up to
// the configured backend.
func Init() error {
	switch setting.Websocket.PubsubType {
	case setting.PubsubTypeMemory:
		DefaultBroker = NewMemoryBroker()
	case setting.PubsubTypeRedis:
		b, err := NewRedisBroker(setting.Websocket.PubsubConnStr)
		if err != nil {
			return fmt.Errorf("pubsub: init redis backend: %w", err)
		}
		DefaultBroker = b
	default:
		return fmt.Errorf("pubsub: unknown PUBSUB_TYPE %q", setting.Websocket.PubsubType)
	}
	return nil
}
