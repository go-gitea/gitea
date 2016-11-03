// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"gopkg.in/macaron.v1"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/base"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/cron"
	"github.com/go-gitea/gitea/modules/process"
	"github.com/go-gitea/gitea/modules/setting"
)

const (
	DASHBOARD base.TplName = "admin/dashboard"
	CONFIG    base.TplName = "admin/config"
	MONITOR   base.TplName = "admin/monitor"
)

var (
	startTime = time.Now()
)

var sysStatus struct {
	Uptime       string
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
	sysStatus.Uptime = base.TimeSincePro(startTime)

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

// Operation types.
type AdminOperation int

const (
	CLEAN_INACTIVATE_USER AdminOperation = iota + 1
	CLEAN_REPO_ARCHIVES
	CLEAN_MISSING_REPOS
	GIT_GC_REPOS
	SYNC_SSH_AUTHORIZED_KEY
	SYNC_REPOSITORY_UPDATE_HOOK
	REINIT_MISSING_REPOSITORY
)

func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminDashboard"] = true

	// Run operation.
	op, _ := com.StrTo(ctx.Query("op")).Int()
	if op > 0 {
		var err error
		var success string

		switch AdminOperation(op) {
		case CLEAN_INACTIVATE_USER:
			success = ctx.Tr("admin.dashboard.delete_inactivate_accounts_success")
			err = models.DeleteInactivateUsers()
		case CLEAN_REPO_ARCHIVES:
			success = ctx.Tr("admin.dashboard.delete_repo_archives_success")
			err = models.DeleteRepositoryArchives()
		case CLEAN_MISSING_REPOS:
			success = ctx.Tr("admin.dashboard.delete_missing_repos_success")
			err = models.DeleteMissingRepositories()
		case GIT_GC_REPOS:
			success = ctx.Tr("admin.dashboard.git_gc_repos_success")
			err = models.GitGcRepos()
		case SYNC_SSH_AUTHORIZED_KEY:
			success = ctx.Tr("admin.dashboard.resync_all_sshkeys_success")
			err = models.RewriteAllPublicKeys()
		case SYNC_REPOSITORY_UPDATE_HOOK:
			success = ctx.Tr("admin.dashboard.resync_all_update_hooks_success")
			err = models.RewriteRepositoryUpdateHook()
		case REINIT_MISSING_REPOSITORY:
			success = ctx.Tr("admin.dashboard.reinit_missing_repos_success")
			err = models.ReinitMissingRepositories()
		}

		if err != nil {
			ctx.Flash.Error(err.Error())
		} else {
			ctx.Flash.Success(success)
		}
		ctx.Redirect(setting.AppSubUrl + "/admin")
		return
	}

	ctx.Data["Stats"] = models.GetStatistic()
	// FIXME: update periodically
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.HTML(200, DASHBOARD)
}

func SendTestMail(ctx *context.Context) {
	email := ctx.Query("email")
	// Send a test email to the user's email address and redirect back to Config
	if err := models.SendTestMail(email); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.test_mail_failed", email, err))
	} else {
		ctx.Flash.Info(ctx.Tr("admin.config.test_mail_sent", email))
	}

	ctx.Redirect(setting.AppSubUrl + "/admin/config")
}

func Config(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.config")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminConfig"] = true

	ctx.Data["AppUrl"] = setting.AppUrl
	ctx.Data["Domain"] = setting.Domain
	ctx.Data["OfflineMode"] = setting.OfflineMode
	ctx.Data["DisableRouterLog"] = setting.DisableRouterLog
	ctx.Data["RunUser"] = setting.RunUser
	ctx.Data["RunMode"] = strings.Title(macaron.Env)
	ctx.Data["RepoRootPath"] = setting.RepoRootPath
	ctx.Data["StaticRootPath"] = setting.StaticRootPath
	ctx.Data["LogRootPath"] = setting.LogRootPath
	ctx.Data["ScriptType"] = setting.ScriptType
	ctx.Data["ReverseProxyAuthUser"] = setting.ReverseProxyAuthUser

	ctx.Data["SSH"] = setting.SSH

	ctx.Data["Service"] = setting.Service
	ctx.Data["DbCfg"] = models.DbCfg
	ctx.Data["Webhook"] = setting.Webhook

	ctx.Data["MailerEnabled"] = false
	if setting.MailService != nil {
		ctx.Data["MailerEnabled"] = true
		ctx.Data["Mailer"] = setting.MailService
	}

	ctx.Data["CacheAdapter"] = setting.CacheAdapter
	ctx.Data["CacheInterval"] = setting.CacheInterval
	ctx.Data["CacheConn"] = setting.CacheConn

	ctx.Data["SessionConfig"] = setting.SessionConfig

	ctx.Data["DisableGravatar"] = setting.DisableGravatar
	ctx.Data["EnableFederatedAvatar"] = setting.EnableFederatedAvatar

	ctx.Data["Git"] = setting.Git

	type logger struct {
		Mode, Config string
	}
	loggers := make([]*logger, len(setting.LogModes))
	for i := range setting.LogModes {
		loggers[i] = &logger{setting.LogModes[i], setting.LogConfigs[i]}
	}
	ctx.Data["Loggers"] = loggers

	ctx.HTML(200, CONFIG)
}

func Monitor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Processes"] = process.Processes
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.HTML(200, MONITOR)
}
