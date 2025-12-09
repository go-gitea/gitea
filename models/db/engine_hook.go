// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/gtprof"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm/contexts"
)

type EngineHook struct {
	Threshold time.Duration
	Logger    log.Logger
}

var _ contexts.Hook = (*EngineHook)(nil)

func (*EngineHook) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	ctx, _ := gtprof.GetTracer().Start(c.Ctx, gtprof.TraceSpanDatabase)
	return ctx, nil
}

func (h *EngineHook) AfterProcess(c *contexts.ContextHook) error {
	span := gtprof.GetContextSpan(c.Ctx)
	if span != nil {
		// Do not record SQL parameters here:
		// * It shouldn't expose the parameters because they contain sensitive information, end users need to report the trace details safely.
		// * Some parameters contain quite long texts, waste memory and are difficult to display.
		span.SetAttributeString(gtprof.TraceAttrDbSQL, c.SQL)
		span.End()
	} else {
		setting.PanicInDevOrTesting("span in database engine hook is nil")
	}
	if c.ExecuteTime >= h.Threshold {
		// 8 is the amount of skips passed to runtime.Caller, so that in the log the correct function
		// is being displayed (the function that ultimately wants to execute the query in the code)
		// instead of the function of the slow query hook being called.
		h.Logger.Log(8, &log.Event{Level: log.WARN}, "[Slow SQL Query] %s %v - %v", c.SQL, c.Args, c.ExecuteTime)
	}
	return nil
}
