// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

var projectPackagePrefix string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	projectPackagePrefix = strings.TrimSuffix(filename, "modules/log/init.go")
	if projectPackagePrefix == filename {
		// in case the source code file is moved, we can not trim the suffix, the code above should also be updated.
		panic("unable to detect correct package prefix, please update file: " + filename)
	}

	rotatingfilewriter.ErrorPrintf = FallbackErrorf

	process.TraceCallback = func(skip int, start bool, pid process.IDType, description string, parentPID process.IDType, typ string) {
		if start && parentPID != "" {
			Log(skip+1, TRACE, "Start %s: %s (from %s) (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(parentPID, FgYellow), NewColoredValue(typ, Reset))
		} else if start {
			Log(skip+1, TRACE, "Start %s: %s (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(typ, Reset))
		} else {
			Log(skip+1, TRACE, "Done %s: %s", NewColoredValue(pid, FgHiYellow), NewColoredValue(description, Reset))
		}
	}
}

func newProcessTypedContext(parent context.Context, desc string) (ctx context.Context, cancel context.CancelFunc) {
	// the "process manager" also calls "log.Trace()" to output logs, so if we want to create new contexts by the manager, we need to disable the trace temporarily
	process.TraceLogDisable(true)
	defer process.TraceLogDisable(false)
	ctx, _, cancel = process.GetManager().AddTypedContext(parent, desc, process.SystemProcessType, false)
	return ctx, cancel
}
