// +build windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

package graceful

import (
	"context"
	"os"
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

type gracefulManager struct {
	ctx                    context.Context
	isChild                bool
	lock                   *sync.RWMutex
	state                  state
	shutdown               chan struct{}
	hammer                 chan struct{}
	terminate              chan struct{}
	runningServerWaitGroup sync.WaitGroup
	createServerWaitGroup  sync.WaitGroup
	terminateWaitGroup     sync.WaitGroup
}

func newGracefulManager(ctx context.Context) *gracefulManager {
	manager := &gracefulManager{
		isChild: false,
		lock:    &sync.RWMutex{},
		ctx:     ctx,
	}
	manager.createServerWaitGroup.Add(numberOfServersToCreate)
	manager.Run()
	return manager
}

func (g *gracefulManager) Run() {
	g.setState(stateRunning)
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_MINWINSVC")); skip {
		return
	}
	run := svc.Run
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Error("Unable to ascertain if running as an Interactive Session: %v", err)
		return
	}
	if isInteractive {
		run = debug.Run
	}
	go run(WindowsServiceName, g)
}

// Execute makes gracefulManager implement svc.Handler
func (g *gracefulManager) Execute(args []string, changes <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	if setting.StartupTimeout > 0 {
		status <- svc.Status{State: svc.StartPending}
	} else {
		status <- svc.Status{State: svc.StartPending, WaitHint: uint32(setting.StartupTimeout / time.Millisecond)}
	}

	// Now need to wait for everything to start...
	if !g.awaitServer(setting.StartupTimeout) {
		return false, 1
	}

	// We need to implement some way of svc.AcceptParamChange/svc.ParamChange
	status <- svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown | acceptHammerCode,
	}

	waitTime := 30 * time.Second

loop:
	for {
		select {
		case <-g.ctx.Done():
			g.doShutdown()
			waitTime += setting.GracefulHammerTime
			break loop
		case change := <-changes:
			switch change.Cmd {
			case svc.Interrogate:
				status <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				g.doShutdown()
				waitTime += setting.GracefulHammerTime
				break loop
			case hammerCode:
				g.doShutdown()
				g.doHammerTime(0 * time.Second)
				break loop
			default:
				log.Debug("Unexpected control request: %v", change.Cmd)
			}
		}
	}
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
				status <- change.CurrentStatus
			case svc.Stop, svc.Shutdown, hammerCmd:
				g.doHammerTime(0 * time.Second)
				break hammerLoop
			default:
				log.Debug("Unexpected control request: %v", change.Cmd)
			}
		case <-g.hammer:
			break hammerLoop
		}
	}
	return false, 0
}

func (g *gracefulManager) RegisterServer() {
	g.runningServerWaitGroup.Add(1)
}

func (g *gracefulManager) awaitServer(limit time.Duration) bool {
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
