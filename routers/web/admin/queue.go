// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

// Queue shows details for a specific queue
func Queue(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.monitor.queue", mq.GetName())
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Queue"] = mq
	ctx.HTML(http.StatusOK, tplQueueManage)
}

// QueueSet sets the maximum number of workers and other settings for this queue
func QueueSet(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	maxNumberStr := ctx.FormString("max-number")

	var err error
	var maxNumber int
	if len(maxNumberStr) > 0 {
		maxNumber, err = strconv.Atoi(maxNumberStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.maxnumberworkers.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
		if maxNumber < -1 {
			maxNumber = -1
		}
	} else {
		maxNumber = mq.GetWorkerMaxNumber()
	}

	mq.SetWorkerMaxNumber(maxNumber)
	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.settings.changed"))
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}
