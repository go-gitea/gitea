// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

func Init() {
	if !setting.Actions.Enabled {
		return
	}

	jobEmitterQueue = queue.CreateUniqueQueue("actions_ready_job", jobEmitterQueueHandler)
	go graceful.GetManager().RunWithShutdownFns(jobEmitterQueue.Run)

	notification.RegisterNotifier(NewNotifier())
}
