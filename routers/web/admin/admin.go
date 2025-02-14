// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/updatechecker"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/cron"
	"code.gitea.io/gitea/services/forms"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplDashboard    templates.TplName = "admin/dashboard"
	tplSystemStatus templates.TplName = "admin/system_status"
	tplSelfCheck    templates.TplName = "admin/self_check"
	tplCron         templates.TplName = "admin/cron"
	tplQueue        templates.TplName = "admin/queue"
	tplPerfTrace    templates.TplName = "admin/perftrace"
	tplStacktrace   templates.TplName = "admin/stacktrace"
	tplQueueManage  templates.TplName = "admin/queue_manage"
	tplStats        templates.TplName = "admin/stats"
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
	LastGCTime   string // last run time
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
	sysStatus.LastGCTime = time.Unix(0, int64(m.LastGC)).Format(time.RFC3339)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

func prepareStartupProblemsAlert(ctx *context.Context) {
	if len(setting.StartupProblems) > 0 {
		content := setting.StartupProblems[0]
		if len(setting.StartupProblems) > 1 {
			content += fmt.Sprintf(" (and %d more)", len(setting.StartupProblems)-1)
		}
		ctx.Flash.Error(content, true)
	}
}

// Dashboard show admin panel dashboard
func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["NeedUpdate"] = updatechecker.GetNeedUpdate(ctx)
	ctx.Data["RemoteVersion"] = updatechecker.GetRemoteVersion(ctx)
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.Data["SSH"] = setting.SSH
	prepareStartupProblemsAlert(ctx)
	ctx.HTML(http.StatusOK, tplDashboard)
}

func SystemStatus(ctx *context.Context) {
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.HTML(http.StatusOK, tplSystemStatus)
}

// DashboardPost run an admin operation
func DashboardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminDashboardForm)
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	// Run operation.
	if form.Op != "" {
		switch form.Op {
		case "sync_repo_branches":
			go func() {
				if err := repo_service.AddAllRepoBranchesToSyncQueue(graceful.GetManager().ShutdownContext()); err != nil {
					log.Error("AddAllRepoBranchesToSyncQueue: %v: %v", ctx.Doer.ID, err)
				}
			}()
			ctx.Flash.Success(ctx.Tr("admin.dashboard.sync_branch.started"))
		case "sync_repo_tags":
			go func() {
				if err := release_service.AddAllRepoTagsToSyncQueue(graceful.GetManager().ShutdownContext()); err != nil {
					log.Error("AddAllRepoTagsToSyncQueue: %v: %v", ctx.Doer.ID, err)
				}
			}()
			ctx.Flash.Success(ctx.Tr("admin.dashboard.sync_tag.started"))
		default:
			task := cron.GetTask(form.Op)
			if task != nil {
				go task.RunWithUser(ctx.Doer, nil)
				ctx.Flash.Success(ctx.Tr("admin.dashboard.task.started", ctx.Tr("admin.dashboard."+form.Op)))
			} else {
				ctx.Flash.Error(ctx.Tr("admin.dashboard.task.unknown", form.Op))
			}
		}
	}
	if form.From == "monitor" {
		ctx.Redirect(setting.AppSubURL + "/-/admin/monitor/cron")
	} else {
		ctx.Redirect(setting.AppSubURL + "/-/admin")
	}
}

func SelfCheck(ctx *context.Context) {
	ctx.Data["PageIsAdminSelfCheck"] = true

	ctx.Data["StartupProblems"] = setting.StartupProblems
	if len(setting.StartupProblems) == 0 && !setting.IsProd {
		if time.Now().Unix()%2 == 0 {
			ctx.Data["StartupProblems"] = []string{"This is a test warning message in dev mode"}
		}
	}

	r, err := db.CheckCollationsDefaultEngine()
	if err != nil {
		ctx.Flash.Error(fmt.Sprintf("CheckCollationsDefaultEngine: %v", err), true)
	}

	if r != nil {
		ctx.Data["DatabaseType"] = setting.Database.Type
		ctx.Data["DatabaseCheckResult"] = r
		hasProblem := false
		if !r.CollationEquals(r.DatabaseCollation, r.ExpectedCollation) {
			ctx.Data["DatabaseCheckCollationMismatch"] = true
			hasProblem = true
		}
		if !r.IsCollationCaseSensitive(r.DatabaseCollation) {
			ctx.Data["DatabaseCheckCollationCaseInsensitive"] = true
			hasProblem = true
		}
		ctx.Data["DatabaseCheckInconsistentCollationColumns"] = r.InconsistentCollationColumns
		hasProblem = hasProblem || len(r.InconsistentCollationColumns) > 0

		ctx.Data["DatabaseCheckHasProblems"] = hasProblem
	}

	elapsed, err := cache.Test()
	if err != nil {
		ctx.Data["CacheError"] = err
	} else if elapsed > cache.SlowCacheThreshold {
		ctx.Data["CacheSlow"] = fmt.Sprint(elapsed)
	}

	ctx.HTML(http.StatusOK, tplSelfCheck)
}

func SelfCheckPost(ctx *context.Context) {
	var problems []string
	frontendAppURL := ctx.FormString("location_origin") + setting.AppSubURL + "/"
	ctxAppURL := httplib.GuessCurrentAppURL(ctx)
	if !strings.HasPrefix(ctxAppURL, frontendAppURL) {
		problems = append(problems, ctx.Locale.TrString("admin.self_check.location_origin_mismatch", frontendAppURL, ctxAppURL))
	}
	ctx.JSON(http.StatusOK, map[string]any{"problems": problems})
}

func CronTasks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.cron")
	ctx.Data["PageIsAdminMonitorCron"] = true
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.HTML(http.StatusOK, tplCron)
}

func MonitorStats(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.stats")
	ctx.Data["PageIsAdminMonitorStats"] = true
	bs, err := json.Marshal(activities_model.GetStatistic(ctx).Counter)
	if err != nil {
		ctx.ServerError("MonitorStats", err)
		return
	}
	statsCounter := map[string]any{}
	err = json.Unmarshal(bs, &statsCounter)
	if err != nil {
		ctx.ServerError("MonitorStats", err)
		return
	}
	statsKeys := make([]string, 0, len(statsCounter))
	for k := range statsCounter {
		if statsCounter[k] == nil {
			continue
		}
		statsKeys = append(statsKeys, k)
	}
	sort.Strings(statsKeys)
	ctx.Data["StatsKeys"] = statsKeys
	ctx.Data["StatsCounter"] = statsCounter
	ctx.HTML(http.StatusOK, tplStats)
}
