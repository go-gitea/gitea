// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strconv"

	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func Queues(ctx *context.Context) {
	if !setting.IsProd {
		initTestQueueOnce()
	}
	ctx.Data["Title"] = ctx.Tr("admin.monitor.queues")
	ctx.Data["PageIsAdminMonitorQueue"] = true
	ctx.Data["Queues"] = queue.GetManager().ManagedQueues()
	ctx.HTML(http.StatusOK, tplQueue)
}

// QueueManage shows details for a specific queue
func QueueManage(ctx *context.Context) {
	qid := ctx.PathParamInt64("qid")
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
	qid := ctx.PathParamInt64("qid")
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
			ctx.Redirect(setting.AppSubURL + "/-/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
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
	ctx.Redirect(setting.AppSubURL + "/-/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

func QueueRemoveAllItems(ctx *context.Context) {
	// Gitea's queue doesn't have transaction support
	// So in rare cases, the queue could be corrupted/out-of-sync
	// Site admin could remove all items from the queue to make it work again
	qid := ctx.PathParamInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	if err := mq.RemoveAllItems(ctx); err != nil {
		ctx.ServerError("RemoveAllItems", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.settings.remove_all_items_done"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}
