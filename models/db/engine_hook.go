// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm/contexts"
)

type SlowQueryHook struct {
	Threshold time.Duration
	Logger    log.Logger
}

var _ contexts.Hook = (*SlowQueryHook)(nil)

func (*SlowQueryHook) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	return c.Ctx, nil
}

func (h *SlowQueryHook) AfterProcess(c *contexts.ContextHook) error {
	if c.ExecuteTime >= h.Threshold {
		// 8 is the amount of skips passed to runtime.Caller, so that in the log the correct function
		// is being displayed (the function that ultimately wants to execute the query in the code)
		// instead of the function of the slow query hook being called.
		h.Logger.Log(8, log.WARN, "[Slow SQL Query] %s %v - %v", c.SQL, c.Args, c.ExecuteTime)
	}
	return nil
}
