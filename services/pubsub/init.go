// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// Init replaces DefaultBroker according to setting.Pubsub. Called from
// routers/init.go before websocket_service.Init so subscribers wire up to the
// configured backend.
func Init() error {
	switch setting.Pubsub.Type {
	case "memory":
		DefaultBroker = NewMemoryBroker()
	case "redis":
		b, err := NewRedisBroker(setting.Pubsub.ConnStr)
		if err != nil {
			return fmt.Errorf("pubsub: init redis backend: %w", err)
		}
		DefaultBroker = b
	default:
		return fmt.Errorf("pubsub: unknown TYPE %q", setting.Pubsub.Type)
	}
	return nil
}
