// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
	"runtime/pprof"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

type state uint8

const (
	stateInit state = iota
	stateRunning
	stateShuttingDown
	stateTerminate
)

type RunCanceler interface {
	Run()
	Cancel()
}

// There are some places that could inherit sockets:
//
// * HTTP or HTTPS main listener
// * HTTP or HTTPS install listener
// * HTTP redirection fallback
// * Builtin SSH listener
//
// If you add a new place you must increment this number
// and add a function to call manager.InformCleanup if it's not going to be used
const numberOfServersToCreate = 4

var (
	manager  *Manager
	initOnce sync.Once
)

// GetManager returns the Manager
func GetManager() *Manager {
	InitManager(context.Background())
	return manager
}

// InitManager creates the graceful manager in the provided context
func InitManager(ctx context.Context) {
	initOnce.Do(func() {
		manager = newGracefulManager(ctx)

		// Set the process default context to the HammerContext
		process.DefaultContext = manager.HammerContext()
	})
}

// RunWithCancel helps to run a function with a custom context, the Cancel function will be called at shutdown
// The Cancel function should stop the Run function in predictable time.
func (g *Manager) RunWithCancel(rc RunCanceler) {
	g.RunAtShutdown(context.Background(), rc.Cancel)
	g.runningServerWaitGroup.Add(1)
	defer g.runningServerWaitGroup.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Critical("PANIC during RunWithCancel: %v\nStacktrace: %s", err, log.Stack(2))
			g.doShutdown()
		}
	}()
	rc.Run()
}

// RunWithShutdownContext takes a function that has a context to watch for shutdown.
// After the provided context is Done(), the main function must return once shutdown is complete.
// (Optionally the HammerContext may be obtained and waited for however, this should be avoided if possible.)
func (g *Manager) RunWithShutdownContext(run func(context.Context)) {
	g.runningServerWaitGroup.Add(1)
	defer g.runningServerWaitGroup.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Critical("PANIC during RunWithShutdownContext: %v\nStacktrace: %s", err, log.Stack(2))
			g.doShutdown()
		}
	}()
	ctx := g.ShutdownContext()
	pprof.SetGoroutineLabels(ctx) // We don't have a label to restore back to but I think this is fine
	run(ctx)
}

// RunAtTerminate adds to the terminate wait group and creates a go-routine to run the provided function at termination
func (g *Manager) RunAtTerminate(terminate func()) {
	g.terminateWaitGroup.Add(1)
	g.lock.Lock()
	defer g.lock.Unlock()
	g.toRunAtTerminate = append(g.toRunAtTerminate,
		func() {
			defer g.terminateWaitGroup.Done()
			defer func() {
				if err := recover(); err != nil {
					log.Critical("PANIC during RunAtTerminate: %v\nStacktrace: %s", err, log.Stack(2))
				}
			}()
			terminate()
		})
}

// RunAtShutdown creates a go-routine to run the provided function at shutdown
func (g *Manager) RunAtShutdown(ctx context.Context, shutdown func()) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.toRunAtShutdown = append(g.toRunAtShutdown,
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Critical("PANIC during RunAtShutdown: %v\nStacktrace: %s", err, log.Stack(2))
				}
			}()
			select {
			case <-ctx.Done():
				return
			default:
				shutdown()
			}
		})
}

func (g *Manager) doShutdown() {
	if !g.setStateTransition(stateRunning, stateShuttingDown) {
		g.DoImmediateHammer()
		return
	}
	g.lock.Lock()
	g.shutdownCtxCancel()
	atShutdownCtx := pprof.WithLabels(g.hammerCtx, pprof.Labels("graceful-lifecycle", "post-shutdown"))
	pprof.SetGoroutineLabels(atShutdownCtx)
	for _, fn := range g.toRunAtShutdown {
		go fn()
	}
	g.lock.Unlock()

	if setting.GracefulHammerTime >= 0 {
		go g.doHammerTime(setting.GracefulHammerTime)
	}
	go func() {
		g.runningServerWaitGroup.Wait()
		// Mop up any remaining unclosed events.
		g.doHammerTime(0)
		<-time.After(1 * time.Second)
		g.doTerminate()
		g.terminateWaitGroup.Wait()
		g.lock.Lock()
		g.managerCtxCancel()
		g.lock.Unlock()
	}()
}

func (g *Manager) doHammerTime(d time.Duration) {
	time.Sleep(d)
	g.lock.Lock()
	select {
	case <-g.hammerCtx.Done():
	default:
		log.Warn("Setting Hammer condition")
		g.hammerCtxCancel()
		atHammerCtx := pprof.WithLabels(g.terminateCtx, pprof.Labels("graceful-lifecycle", "post-hammer"))
		pprof.SetGoroutineLabels(atHammerCtx)
	}
	g.lock.Unlock()
}

func (g *Manager) doTerminate() {
	if !g.setStateTransition(stateShuttingDown, stateTerminate) {
		return
	}
	g.lock.Lock()
	select {
	case <-g.terminateCtx.Done():
	default:
		log.Warn("Terminating")
		g.terminateCtxCancel()
		atTerminateCtx := pprof.WithLabels(g.managerCtx, pprof.Labels("graceful-lifecycle", "post-terminate"))
		pprof.SetGoroutineLabels(atTerminateCtx)

		for _, fn := range g.toRunAtTerminate {
			go fn()
		}
	}
	g.lock.Unlock()
}

// IsChild returns if the current process is a child of previous Gitea process
func (g *Manager) IsChild() bool {
	return g.isChild
}

// IsShutdown returns a channel which will be closed at shutdown.
// The order of closure is shutdown, hammer (potentially), terminate
func (g *Manager) IsShutdown() <-chan struct{} {
	return g.shutdownCtx.Done()
}

// IsHammer returns a channel which will be closed at hammer.
// Servers running within the running server wait group should respond to IsHammer
// if not shutdown already
func (g *Manager) IsHammer() <-chan struct{} {
	return g.hammerCtx.Done()
}

// ServerDone declares a running server done and subtracts one from the
// running server wait group. Users probably do not want to call this
// and should use one of the RunWithShutdown* functions
func (g *Manager) ServerDone() {
	g.runningServerWaitGroup.Done()
}

func (g *Manager) setStateTransition(old, new state) bool {
	g.lock.Lock()
	if g.state != old {
		g.lock.Unlock()
		return false
	}
	g.state = new
	g.lock.Unlock()
	return true
}

// InformCleanup tells the cleanup wait group that we have either taken a listener or will not be taking a listener.
// At the moment the total number of servers (numberOfServersToCreate) are pre-defined as a const before global init,
// so this function MUST be called if a server is not used.
func (g *Manager) InformCleanup() {
	g.createServerWaitGroup.Done()
}

// Done allows the manager to be viewed as a context.Context, it returns a channel that is closed when the server is finished terminating
func (g *Manager) Done() <-chan struct{} {
	return g.managerCtx.Done()
}

// Err allows the manager to be viewed as a context.Context done at Terminate
func (g *Manager) Err() error {
	return g.managerCtx.Err()
}

// Value allows the manager to be viewed as a context.Context done at Terminate
func (g *Manager) Value(key any) any {
	return g.managerCtx.Value(key)
}

// Deadline returns nil as there is no fixed Deadline for the manager, it allows the manager to be viewed as a context.Context
func (g *Manager) Deadline() (deadline time.Time, ok bool) {
	return g.managerCtx.Deadline()
}
