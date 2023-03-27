// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/notify"
)

func Init() {
	if !setting.Actions.Enabled {
		return
	}

	jobEmitterQueue = queue.CreateUniqueQueue("actions_ready_job", jobEmitterQueueHandle, new(jobUpdate))
	go graceful.GetManager().RunWithShutdownFns(jobEmitterQueue.Run)

	notify.RegisterNotifier(NewNotifier())
}
