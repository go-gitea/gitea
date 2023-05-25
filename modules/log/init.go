// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"runtime"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

var (
	projectPackagePrefix string
	processTraceDisabled atomic.Int64
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	projectPackagePrefix = strings.TrimSuffix(filename, "modules/log/init.go")
	if projectPackagePrefix == filename {
		// in case the source code file is moved, we can not trim the suffix, the code above should also be updated.
		panic("unable to detect correct package prefix, please update file: " + filename)
	}

	rotatingfilewriter.ErrorPrintf = FallbackErrorf

	process.Trace = func(start bool, pid process.IDType, description string, parentPID process.IDType, typ string) {
		// the logger manager has its own mutex lock, so it's safe to use "Load" here
		if processTraceDisabled.Load() != 0 {
			return
		}
		if start && parentPID != "" {
			Log(1, TRACE, "Start %s: %s (from %s) (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(parentPID, FgYellow), NewColoredValue(typ, Reset))
		} else if start {
			Log(1, TRACE, "Start %s: %s (%s)", NewColoredValue(pid, FgHiYellow), description, NewColoredValue(typ, Reset))
		} else {
			Log(1, TRACE, "Done %s: %s", NewColoredValue(pid, FgHiYellow), NewColoredValue(description, Reset))
		}
	}
}

func newContext(parent context.Context, desc string) (ctx context.Context, cancel context.CancelFunc) {
	// the "process manager" also calls "log.Trace()" to output logs, so if we want to create new contexts by the manager, we need to disable the trace temporarily
	processTraceDisabled.Add(1)
	defer processTraceDisabled.Add(-1)
	ctx, _, cancel = process.GetManager().AddTypedContext(parent, desc, process.SystemProcessType, false)
	return ctx, cancel
}
