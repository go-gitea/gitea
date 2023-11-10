// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
)

func Init() {
	if !setting.Actions.Enabled {
		return
	}

	jobEmitterQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "actions_ready_job", jobEmitterQueueHandler)
	if jobEmitterQueue == nil {
		log.Fatal("Unable to create actions_ready_job queue")
	}
	go graceful.GetManager().RunWithCancel(jobEmitterQueue)

	notify_service.RegisterNotifier(NewNotifier())
}
