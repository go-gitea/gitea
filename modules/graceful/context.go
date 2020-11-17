// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"context"
	"fmt"
	"time"
)

// Errors for context.Err()
var (
	ErrShutdown  = fmt.Errorf("Graceful Manager called Shutdown")
	ErrHammer    = fmt.Errorf("Graceful Manager called Hammer")
	ErrTerminate = fmt.Errorf("Graceful Manager called Terminate")
)

// ChannelContext is a context that wraps a channel and error as a context
type ChannelContext struct {
	done <-chan struct{}
	err  error
}

// NewChannelContext creates a ChannelContext from a channel and error
func NewChannelContext(done <-chan struct{}, err error) *ChannelContext {
	return &ChannelContext{
		done: done,
		err:  err,
	}
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. There is no Deadline for a ChannelContext
func (ctx *ChannelContext) Deadline() (deadline time.Time, ok bool) {
	return
}

// Done returns the channel provided at the creation of this context.
// When closed, work done on behalf of this context should be canceled.
func (ctx *ChannelContext) Done() <-chan struct{} {
	return ctx.done
}

// Err returns nil, if Done is not closed. If Done is closed,
// Err returns the error provided at the creation of this context
func (ctx *ChannelContext) Err() error {
	select {
	case <-ctx.done:
		return ctx.err
	default:
		return nil
	}
}

// Value returns nil for all calls as no values are or can be associated with this context
func (ctx *ChannelContext) Value(key interface{}) interface{} {
	return nil
}

// ShutdownContext returns a context.Context that is Done at shutdown
// Callers using this context should ensure that they are registered as a running server
// in order that they are waited for.
func (g *Manager) ShutdownContext() context.Context {
	return &ChannelContext{
		done: g.IsShutdown(),
		err:  ErrShutdown,
	}
}

// HammerContext returns a context.Context that is Done at hammer
// Callers using this context should ensure that they are registered as a running server
// in order that they are waited for.
func (g *Manager) HammerContext() context.Context {
	return &ChannelContext{
		done: g.IsHammer(),
		err:  ErrHammer,
	}
}

// TerminateContext returns a context.Context that is Done at terminate
// Callers using this context should ensure that they are registered as a terminating server
// in order that they are waited for.
func (g *Manager) TerminateContext() context.Context {
	return &ChannelContext{
		done: g.IsTerminate(),
		err:  ErrTerminate,
	}
}
