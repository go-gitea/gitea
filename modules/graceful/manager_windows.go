// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

//go:build windows

package graceful

import (
	"context"
	"os"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

// WindowsServiceName is the name of the Windows service
var WindowsServiceName = "gitea"

const (
	hammerCode       = 128
	hammerCmd        = svc.Cmd(hammerCode)
	acceptHammerCode = svc.Accepted(hammerCode)
)

// Manager manages the graceful shutdown process
type Manager struct {
	ctx                    context.Context
	isChild                bool
	lock                   *sync.RWMutex
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
	createServerWaitGroup  sync.WaitGroup
	terminateWaitGroup     sync.WaitGroup
	shutdownRequested      chan struct{}

	toRunAtShutdown  []func()
	toRunAtTerminate []func()
}

func newGracefulManager(ctx context.Context) *Manager {
	manager := &Manager{
		isChild: false,
		lock:    &sync.RWMutex{},
		ctx:     ctx,
	}
	manager.createServerWaitGroup.Add(numberOfServersToCreate)
	manager.start()
	return manager
}

func (g *Manager) start() {
	// Make contexts
	g.terminateCtx, g.terminateCtxCancel = context.WithCancel(g.ctx)
	g.shutdownCtx, g.shutdownCtxCancel = context.WithCancel(g.ctx)
	g.hammerCtx, g.hammerCtxCancel = context.WithCancel(g.ctx)
	g.managerCtx, g.managerCtxCancel = context.WithCancel(g.ctx)

	// Next add pprof labels to these contexts
	g.terminateCtx = pprof.WithLabels(g.terminateCtx, pprof.Labels("graceful-lifecycle", "with-terminate"))
	g.shutdownCtx = pprof.WithLabels(g.shutdownCtx, pprof.Labels("graceful-lifecycle", "with-shutdown"))
	g.hammerCtx = pprof.WithLabels(g.hammerCtx, pprof.Labels("graceful-lifecycle", "with-hammer"))
	g.managerCtx = pprof.WithLabels(g.managerCtx, pprof.Labels("graceful-lifecycle", "with-manager"))

	// Now label this and all goroutines created by this goroutine with the graceful-lifecycle manager
	pprof.SetGoroutineLabels(g.managerCtx)
	defer pprof.SetGoroutineLabels(g.ctx)

	// Make channels
	g.shutdownRequested = make(chan struct{})

	// Set the running state
	g.setState(stateRunning)
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_MINWINSVC")); skip {
		log.Trace("Skipping SVC check as SKIP_MINWINSVC is set")
		return
	}

	// Make SVC process
	run := svc.Run

	//lint:ignore SA1019 We use IsAnInteractiveSession because IsWindowsService has a different permissions profile
	isAnInteractiveSession, err := svc.IsAnInteractiveSession() //nolint:staticcheck
	if err != nil {
		log.Error("Unable to ascertain if running as an Windows Service: %v", err)
		return
	}
	if isAnInteractiveSession {
		log.Trace("Not running a service ... using the debug SVC manager")
		run = debug.Run
	}
	go func() {
		_ = run(WindowsServiceName, g)
	}()
}

// Execute makes Manager implement svc.Handler
func (g *Manager) Execute(args []string, changes <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	if setting.StartupTimeout > 0 {
		status <- svc.Status{State: svc.StartPending, WaitHint: uint32(setting.StartupTimeout / time.Millisecond)}
	} else {
		status <- svc.Status{State: svc.StartPending}
	}

	log.Trace("Awaiting server start-up")
	// Now need to wait for everything to start...
	if !g.awaitServer(setting.StartupTimeout) {
		log.Trace("... start-up failed ... Stopped")
		return false, 1
	}

	log.Trace("Sending Running state to SVC")

	// We need to implement some way of svc.AcceptParamChange/svc.ParamChange
	status <- svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown | acceptHammerCode,
	}

	log.Trace("Started")

	waitTime := 30 * time.Second

loop:
	for {
		select {
		case <-g.ctx.Done():
			log.Trace("Shutting down")
			g.DoGracefulShutdown()
			waitTime += setting.GracefulHammerTime
			break loop
		case <-g.shutdownRequested:
			log.Trace("Shutting down")
			waitTime += setting.GracefulHammerTime
			break loop
		case change := <-changes:
			switch change.Cmd {
			case svc.Interrogate:
				log.Trace("SVC sent interrogate")
				status <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Trace("SVC requested shutdown - shutting down")
				g.DoGracefulShutdown()
				waitTime += setting.GracefulHammerTime
				break loop
			case hammerCode:
				log.Trace("SVC requested hammer - shutting down and hammering immediately")
				g.DoGracefulShutdown()
				g.DoImmediateHammer()
				break loop
			default:
				log.Debug("Unexpected control request: %v", change.Cmd)
			}
		}
	}

	log.Trace("Sending StopPending state to SVC")
	status <- svc.Status{
		State:    svc.StopPending,
		WaitHint: uint32(waitTime / time.Millisecond),
	}

hammerLoop:
	for {
		select {
		case change := <-changes:
			switch change.Cmd {
			case svc.Interrogate:
				log.Trace("SVC sent interrogate")
				status <- change.CurrentStatus
			case svc.Stop, svc.Shutdown, hammerCmd:
				log.Trace("SVC requested hammer - hammering immediately")
				g.DoImmediateHammer()
				break hammerLoop
			default:
				log.Debug("Unexpected control request: %v", change.Cmd)
			}
		case <-g.hammerCtx.Done():
			break hammerLoop
		}
	}

	log.Trace("Stopped")
	return false, 0
}

// DoImmediateHammer causes an immediate hammer
func (g *Manager) DoImmediateHammer() {
	g.doHammerTime(0 * time.Second)
}

// DoGracefulShutdown causes a graceful shutdown
func (g *Manager) DoGracefulShutdown() {
	g.lock.Lock()
	select {
	case <-g.shutdownRequested:
		g.lock.Unlock()
	default:
		close(g.shutdownRequested)
		g.lock.Unlock()
		g.doShutdown()
	}
}

// RegisterServer registers the running of a listening server.
// Any call to RegisterServer must be matched by a call to ServerDone
func (g *Manager) RegisterServer() {
	g.runningServerWaitGroup.Add(1)
}

func (g *Manager) awaitServer(limit time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		g.createServerWaitGroup.Wait()
	}()
	if limit > 0 {
		select {
		case <-c:
			return true // completed normally
		case <-time.After(limit):
			return false // timed out
		case <-g.IsShutdown():
			return false
		}
	} else {
		select {
		case <-c:
			return true // completed normally
		case <-g.IsShutdown():
			return false
		}
	}
}
