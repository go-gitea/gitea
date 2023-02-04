// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"runtime/pprof"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/felixge/fgprof"
)

// PProfProcessStacktrace returns the stacktrace similar to GoroutineStacktrace but without rendering it
func PProfProcessStacktrace(ctx *context.Context) {
	flat := ctx.FormBool("flat")
	noSystem := ctx.FormBool("no-system")

	format := ctx.FormString("format")
	jsonFormat := format == "json"

	start := time.Now()
	filename := "process-stacktrace-" + strconv.FormatInt(start.Unix(), 10)
	if jsonFormat {
		filename += ".json"
	}

	processStacks, processCount, goroutineCount, err := process.GetManager().ProcessStacktraces(flat, noSystem)
	if err != nil {
		ctx.ServerError("ProcessStacktraces", err)
	}

	ctx.SetServeHeaders(&context.ServeHeaderOptions{
		Filename:     filename,
		LastModified: start,
	})

	if jsonFormat {
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"TotalNumberOfGoroutines": goroutineCount,
			"TotalNumberOfProcesses":  processCount,
			"Processes":               processStacks,
		})
		return
	}

	if err := process.WriteProcesses(ctx.Resp, processStacks, processCount, goroutineCount, "", flat); err != nil {
		ctx.ServerError("WriteProcesses", err)
		return
	}
}

// PProfFGProfile returns the Full Go Profile from fgprof
func PProfFGProfile(ctx *context.Context) {
	durationStr := ctx.FormString("duration")
	duration := 30 * time.Second
	if durationStr != "" {
		var err error
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("monitor.pprof.duration_invalid"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor")
			return
		}
	}

	format := fgprof.Format(ctx.FormString("format"))
	if format != fgprof.FormatFolded {
		format = fgprof.FormatPprof
	}

	start := time.Now()

	ctx.SetServeHeaders(&context.ServeHeaderOptions{
		Filename:     "fgprof-profile-" + strconv.FormatInt(start.Unix(), 10),
		LastModified: start,
	})

	fn := fgprof.Start(ctx.Resp, format)

	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}

	err := fn()
	if err != nil {
		ctx.ServerError("fgprof.Write", err)
	}
}

// PProfCPUProfile returns the PProf CPU Profile
func PProfCPUProfile(ctx *context.Context) {
	durationStr := ctx.FormString("duration")
	duration := 30 * time.Second
	if durationStr != "" {
		var err error
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("monitor.pprof.duration_invalid"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor")
			return
		}
	}

	start := time.Now()

	ctx.SetServeHeaders(&context.ServeHeaderOptions{
		Filename:     "cpu-profile-" + strconv.FormatInt(start.Unix(), 10),
		LastModified: start,
	})

	err := pprof.StartCPUProfile(ctx.Resp)
	if err != nil {
		ctx.ServerError("StartCPUProfile", err)
		return
	}

	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}
	pprof.StopCPUProfile()
}

// PProfNamedProfile returns the PProf Profile
func PProfNamedProfile(ctx *context.Context) {
	name := ctx.FormString("name")
	profile := pprof.Lookup(name)
	if profile == nil {
		ctx.ServerError(fmt.Sprintf("pprof.Lookup(%s)", name), fmt.Errorf("missing profile: %s", name))
		return
	}

	debug := ctx.FormInt("debug")

	start := time.Now()

	ctx.SetServeHeaders(&context.ServeHeaderOptions{
		Filename:     name + "-profile-" + strconv.FormatInt(start.Unix(), 10),
		LastModified: start,
	})
	if err := profile.WriteTo(ctx.Resp, debug); err != nil {
		ctx.ServerError(fmt.Sprintf("PProfNamedProfile(%s).WriteTo", name), err)
		return
	}
}
