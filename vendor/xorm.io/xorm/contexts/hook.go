// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package contexts

import (
	"context"
	"database/sql"
	"time"
)

// ContextHook represents a hook context
type ContextHook struct {
	start       time.Time
	Ctx         context.Context
	SQL         string        // log content or SQL
	Args        []interface{} // if it's a SQL, it's the arguments
	Result      sql.Result
	ExecuteTime time.Duration
	Err         error // SQL executed error
}

// NewContextHook return context for hook
func NewContextHook(ctx context.Context, sql string, args []interface{}) *ContextHook {
	return &ContextHook{
		start: time.Now(),
		Ctx:   ctx,
		SQL:   sql,
		Args:  args,
	}
}

// End finish the hook invokation
func (c *ContextHook) End(ctx context.Context, result sql.Result, err error) {
	c.Ctx = ctx
	c.Result = result
	c.Err = err
	c.ExecuteTime = time.Now().Sub(c.start)
}

// Hook represents a hook behaviour
type Hook interface {
	BeforeProcess(c *ContextHook) (context.Context, error)
	AfterProcess(c *ContextHook) error
}

// Hooks implements Hook interface but contains multiple Hook
type Hooks struct {
	hooks []Hook
}

// AddHook adds a Hook
func (h *Hooks) AddHook(hooks ...Hook) {
	h.hooks = append(h.hooks, hooks...)
}

// BeforeProcess invoked before execute the process
func (h *Hooks) BeforeProcess(c *ContextHook) (context.Context, error) {
	ctx := c.Ctx
	for _, h := range h.hooks {
		var err error
		ctx, err = h.BeforeProcess(c)
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

// AfterProcess invoked after exetue the process
func (h *Hooks) AfterProcess(c *ContextHook) error {
	firstErr := c.Err
	for _, h := range h.hooks {
		err := h.AfterProcess(c)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
