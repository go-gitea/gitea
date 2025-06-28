// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

//go:build windows

package graceful

import (
	"os"
	"runtime/pprof"
	"strconv"
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

func (g *Manager) start() {
	// Now label this and all goroutines created by this goroutine with the graceful-lifecycle manager
	pprof.SetGoroutineLabels(g.managerCtx)
	defer pprof.SetGoroutineLabels(g.ctx)

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_MINWINSVC")); skip {
		log.Trace("Skipping SVC check as SKIP_MINWINSVC is set")
		return
	}

	// Make SVC process
	run := svc.Run

	isAnInteractiveSession, err := svc.IsAnInteractiveSession() //nolint:staticcheck // must use IsAnInteractiveSession because IsWindowsService has a different permissions profile
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

func (g *Manager) awaitServer(limit time.Duration) bool {
	c := make(chan struct{})
	go func() {
		g.createServerCond.L.Lock()
		for {
			if g.createdServer >= numberOfServersToCreate {
				g.createServerCond.L.Unlock()
				close(c)
				return
			}
			select {
			case <-g.IsShutdown():
				g.createServerCond.L.Unlock()
				return
			default:
			}
			g.createServerCond.Wait()
		}
	}()

	var tc <-chan time.Time
	if limit > 0 {
		tc = time.After(limit)
	}
	select {
	case <-c:
		return true // completed normally
	case <-tc:
		return false // timed out
	case <-g.IsShutdown():
		g.createServerCond.Signal()
		return false
	}
}

func (g *Manager) notify(msg systemdNotifyMsg) {
	// Windows doesn't use systemd to notify
}

func KillParent() {
	// Windows doesn't need to "kill parent" because there is no graceful restart
}
