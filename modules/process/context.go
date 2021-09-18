// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
)

// Context is a wrapper around context.Context for having the current pid for this context
type Context struct {
	context.Context
	pid int64
}

// GetPID returns the PID for this context
func (c *Context) GetPID() int64 {
	return c.pid
}

// GetParent returns the parent process context if any
func (c *Context) GetParent() *Context {
	return GetContext(c.Context)
}

func (c *Context) Value(key interface{}) interface{} {
	if key == ProcessContextKey {
		return c
	}
	return c.Context.Value(key)
}

// ProcessContextKey is the key under which process contexts are stored
var ProcessContextKey interface{} = "process-context"

// GetContext will return a process context if one exists
func GetContext(ctx context.Context) *Context {
	if pCtx, ok := ctx.(*Context); ok {
		return pCtx
	}
	pCtxInterface := ctx.Value(ProcessContextKey)
	if pCtxInterface == nil {
		return nil
	}
	if pCtx, ok := pCtxInterface.(*Context); ok {
		return pCtx
	}
	return nil
}

// GetPID returns the PID for this context
func GetPID(ctx context.Context) int64 {
	pCtx := GetContext(ctx)
	if pCtx == nil {
		return 0
	}
	return pCtx.GetPID()
}

func GetParentPID(ctx context.Context) int64 {
	parentPID := int64(0)
	if parentProcess := GetContext(ctx); parentProcess != nil {
		parentPID = parentProcess.GetPID()
	}
	return parentPID
}
