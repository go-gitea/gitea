// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
	"runtime/pprof"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/gtprof"
)

// FIXME: it seems that there is a bug when using systemd Type=notify: the "Install Page" (INSTALL_LOCK=false) doesn't notify properly.
// At the moment, no idea whether it also affects Windows Service, or whether it's a regression bug. It needs to be investigated later.

type systemdNotifyMsg string

const (
	readyMsg     systemdNotifyMsg = "READY=1"
	stoppingMsg  systemdNotifyMsg = "STOPPING=1"
	reloadingMsg systemdNotifyMsg = "RELOADING=1"
	watchdogMsg  systemdNotifyMsg = "WATCHDOG=1"
)

func statusMsg(msg string) systemdNotifyMsg {
	return systemdNotifyMsg("STATUS=" + msg)
}

// Manager manages the graceful shutdown process
type Manager struct {
	ctx                    context.Context
	isChild                bool
	forked                 bool
	lock                   sync.RWMutex
	state                  state
	shutdownCtx            context.Context
	hammerCtx              context.Context
	terminateCtx           context.Context
	managerCtx             context.Context
	shutdownCtxCancel      context.CancelFunc
	hammerCtxCancel        context.CancelFunc
	terminateCtxCancel     context.CancelFunc
	managerCtxCancel       context.CancelFunc
	runningServerWaitGroup sync.WaitGroup
	terminateWaitGroup     sync.WaitGroup
	createServerCond       sync.Cond
	createdServer          int
	shutdownRequested      chan struct{}

	toRunAtShutdown  []func()
	toRunAtTerminate []func()
}

func newGracefulManager(ctx context.Context) *Manager {
	manager := &Manager{ctx: ctx, shutdownRequested: make(chan struct{})}
	manager.createServerCond.L = &sync.Mutex{}
	manager.prepare(ctx)
	manager.start()
	return manager
}

func (g *Manager) prepare(ctx context.Context) {
	g.terminateCtx, g.terminateCtxCancel = context.WithCancel(ctx)
	g.shutdownCtx, g.shutdownCtxCancel = context.WithCancel(ctx)
	g.hammerCtx, g.hammerCtxCancel = context.WithCancel(ctx)
	g.managerCtx, g.managerCtxCancel = context.WithCancel(ctx)

	g.terminateCtx = pprof.WithLabels(g.terminateCtx, pprof.Labels(gtprof.LabelGracefulLifecycle, "with-terminate"))
	g.shutdownCtx = pprof.WithLabels(g.shutdownCtx, pprof.Labels(gtprof.LabelGracefulLifecycle, "with-shutdown"))
	g.hammerCtx = pprof.WithLabels(g.hammerCtx, pprof.Labels(gtprof.LabelGracefulLifecycle, "with-hammer"))
	g.managerCtx = pprof.WithLabels(g.managerCtx, pprof.Labels(gtprof.LabelGracefulLifecycle, "with-manager"))

	if !g.setStateTransition(stateInit, stateRunning) {
		panic("invalid graceful manager state: transition from init to running failed")
	}
}

// DoImmediateHammer causes an immediate hammer
func (g *Manager) DoImmediateHammer() {
	g.notify(statusMsg("Sending immediate hammer"))
	g.doHammerTime(0 * time.Second)
}

// DoGracefulShutdown causes a graceful shutdown
func (g *Manager) DoGracefulShutdown() {
	g.lock.Lock()
	select {
	case <-g.shutdownRequested:
	default:
		close(g.shutdownRequested)
	}
	forked := g.forked
	g.lock.Unlock()

	if !forked {
		g.notify(stoppingMsg)
	} else {
		g.notify(statusMsg("Shutting down after fork"))
	}
	g.doShutdown()
}

// RegisterServer registers the running of a listening server, in the case of unix this means that the parent process can now die.
// Any call to RegisterServer must be matched by a call to ServerDone
func (g *Manager) RegisterServer() {
	KillParent()
	g.runningServerWaitGroup.Add(1)
}
