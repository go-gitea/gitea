// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/queue"
)

func Init() {
	jobEmitterQueue = queue.CreateUniqueQueue("bots_ready_job", jobEmitterQueueHandle, new(jobUpdate))
	go graceful.GetManager().RunWithShutdownFns(jobEmitterQueue.Run)
}
