// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	notify_service "code.gitea.io/gitea/services/notify"
)

func Init() error {
	notify_service.RegisterNotifier(&wsNotifier{})
	return nil
}
