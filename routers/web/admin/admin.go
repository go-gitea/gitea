// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/updatechecker"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/cron"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplDashboard  base.TplName = "admin/dashboard"
	tplMonitor    base.TplName = "admin/monitor"
	tplStacktrace base.TplName = "admin/stacktrace"
	tplQueue      base.TplName = "admin/queue"
)

var sysStatus struct {
	StartTime    string
	NumGoroutine int

	// General statistics.
	MemAllocated string // bytes allocated and still in use
	MemTotal     string // bytes allocated (even if freed)
	MemSys       string // bytes obtained from system (sum of XxxSys below)
	Lookups      uint64 // number of pointer lookups
	MemMallocs   uint64 // number of mallocs
	MemFrees     uint64 // number of frees

	// Main allocation heap statistics.
	HeapAlloc    string // bytes allocated and still in use
	HeapSys      string // bytes obtained from system
	HeapIdle     string // bytes in idle spans
	HeapInuse    string // bytes in non-idle span
	HeapReleased string // bytes released to the OS
	HeapObjects  uint64 // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  string // bootstrap stacks
	StackSys    string
	MSpanInuse  string // mspan structures
	MSpanSys    string
	MCacheInuse string // mcache structures
	MCacheSys   string
	BuckHashSys string // profiling bucket hash table
	GCSys       string // GC metadata
	OtherSys    string // other system allocations

	// Garbage collector statistics.
	NextGC       string // next run in HeapAlloc time (bytes)
	LastGC       string // last run in absolute time (ns)
	PauseTotalNs string
	PauseNs      string // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC        uint32
}

func updateSystemStatus() {
	sysStatus.StartTime = setting.AppStartTime.Format(time.RFC3339)

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = base.FileSize(int64(m.Alloc))
	sysStatus.MemTotal = base.FileSize(int64(m.TotalAlloc))
	sysStatus.MemSys = base.FileSize(int64(m.Sys))
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = base.FileSize(int64(m.HeapAlloc))
	sysStatus.HeapSys = base.FileSize(int64(m.HeapSys))
	sysStatus.HeapIdle = base.FileSize(int64(m.HeapIdle))
	sysStatus.HeapInuse = base.FileSize(int64(m.HeapInuse))
	sysStatus.HeapReleased = base.FileSize(int64(m.HeapReleased))
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = base.FileSize(int64(m.StackInuse))
	sysStatus.StackSys = base.FileSize(int64(m.StackSys))
	sysStatus.MSpanInuse = base.FileSize(int64(m.MSpanInuse))
	sysStatus.MSpanSys = base.FileSize(int64(m.MSpanSys))
	sysStatus.MCacheInuse = base.FileSize(int64(m.MCacheInuse))
	sysStatus.MCacheSys = base.FileSize(int64(m.MCacheSys))
	sysStatus.BuckHashSys = base.FileSize(int64(m.BuckHashSys))
	sysStatus.GCSys = base.FileSize(int64(m.GCSys))
	sysStatus.OtherSys = base.FileSize(int64(m.OtherSys))

	sysStatus.NextGC = base.FileSize(int64(m.NextGC))
	sysStatus.LastGC = fmt.Sprintf("%.1fs", float64(time.Now().UnixNano()-int64(m.LastGC))/1000/1000/1000)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

// Dashboard show admin panel dashboard
func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["Stats"] = activities_model.GetStatistic()
	ctx.Data["NeedUpdate"] = updatechecker.GetNeedUpdate()
	ctx.Data["RemoteVersion"] = updatechecker.GetRemoteVersion()
	// FIXME: update periodically
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.Data["SSH"] = setting.SSH
	ctx.HTML(http.StatusOK, tplDashboard)
}

// DashboardPost run an admin operation
func DashboardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminDashboardForm)
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["Stats"] = activities_model.GetStatistic()
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	// Run operation.
	if form.Op != "" {
		task := cron.GetTask(form.Op)
		if task != nil {
			go task.RunWithUser(ctx.Doer, nil)
			ctx.Flash.Success(ctx.Tr("admin.dashboard.task.started", ctx.Tr("admin.dashboard."+form.Op)))
		} else {
			ctx.Flash.Error(ctx.Tr("admin.dashboard.task.unknown", form.Op))
		}
	}
	if form.From == "monitor" {
		ctx.Redirect(setting.AppSubURL + "/admin/monitor")
	} else {
		ctx.Redirect(setting.AppSubURL + "/admin")
	}
}

// Monitor show admin monitor page
func Monitor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Processes"], ctx.Data["ProcessCount"] = process.GetManager().Processes(false, true)
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.Data["Queues"] = queue.GetManager().ManagedQueues()

	ctx.HTML(http.StatusOK, tplMonitor)
}

// GoroutineStacktrace show admin monitor goroutines page
func GoroutineStacktrace(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true

	processStacks, processCount, goroutineCount, err := process.GetManager().ProcessStacktraces(false, false)
	if err != nil {
		ctx.ServerError("GoroutineStacktrace", err)
		return
	}

	ctx.Data["ProcessStacks"] = processStacks

	ctx.Data["GoroutineCount"] = goroutineCount
	ctx.Data["ProcessCount"] = processCount

	ctx.HTML(http.StatusOK, tplStacktrace)
}

// MonitorCancel cancels a process
func MonitorCancel(ctx *context.Context) {
	pid := ctx.Params("pid")
	process.GetManager().Cancel(process.IDType(pid))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/monitor",
	})
}

// Queue shows details for a specific queue
func Queue(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.monitor.queue", mq.Name)
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Queue"] = mq
	ctx.HTML(http.StatusOK, tplQueue)
}

// WorkerCancel cancels a worker group
func WorkerCancel(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	pid := ctx.ParamsInt64("pid")
	mq.CancelWorkers(pid)
	ctx.Flash.Info(ctx.Tr("admin.monitor.queue.pool.cancelling"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10),
	})
}

// Flush flushes a queue
func Flush(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	timeout, err := time.ParseDuration(ctx.FormString("timeout"))
	if err != nil {
		timeout = -1
	}
	ctx.Flash.Info(ctx.Tr("admin.monitor.queue.pool.flush.added", mq.Name))
	go func() {
		err := mq.Flush(timeout)
		if err != nil {
			log.Error("Flushing failure for %s: Error %v", mq.Name, err)
		}
	}()
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// Pause pauses a queue
func Pause(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	mq.Pause()
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// Resume resumes a queue
func Resume(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	mq.Resume()
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// AddWorkers adds workers to a worker group
func AddWorkers(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	number := ctx.FormInt("number")
	if number < 1 {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.addworkers.mustnumbergreaterzero"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	timeout, err := time.ParseDuration(ctx.FormString("timeout"))
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.addworkers.musttimeoutduration"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	if _, ok := mq.Managed.(queue.ManagedPool); !ok {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.none"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	mq.AddWorkers(number, timeout)
	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.pool.added"))
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// SetQueueSettings sets the maximum number of workers and other settings for this queue
func SetQueueSettings(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	if _, ok := mq.Managed.(queue.ManagedPool); !ok {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.none"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}

	maxNumberStr := ctx.FormString("max-number")
	numberStr := ctx.FormString("number")
	timeoutStr := ctx.FormString("timeout")

	var err error
	var maxNumber, number int
	var timeout time.Duration
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
		maxNumber = mq.MaxNumberOfWorkers()
	}

	if len(numberStr) > 0 {
		number, err = strconv.Atoi(numberStr)
		if err != nil || number < 0 {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.numberworkers.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
	} else {
		number = mq.BoostWorkers()
	}

	if len(timeoutStr) > 0 {
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.timeout.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
	} else {
		timeout = mq.BoostTimeout()
	}

	mq.SetPoolSettings(maxNumber, number, timeout)
	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.settings.changed"))
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}
