// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/git"
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

// There are three places that could inherit sockets:
//
// * HTTP or HTTPS main listener
// * HTTP redirection fallback
// * SSH
//
// If you add an additional place you must increment this number
// and add a function to call manager.InformCleanup if it's not going to be used
const numberOfServersToCreate = 3

// Manager represents the graceful server manager interface
var Manager *gracefulManager

func init() {
	Manager = newGracefulManager(context.Background())
	// Set the git default context to the HammerContext
	git.DefaultContext = Manager.HammerContext()
	// Set the process default context to the HammerContext
	process.DefaultContext = Manager.HammerContext()
}

// CallbackWithContext is combined runnable and context to watch to see if the caller has finished
type CallbackWithContext func(ctx context.Context, callback func())

// RunnableWithShutdownFns is a runnable with functions to run at shutdown and terminate
// After the callback to atShutdown is called and is complete, the main function must return.
// Similarly the callback function provided to atTerminate must return once termination is complete.
// Please note that use of the atShutdown and atTerminate callbacks will create go-routines that will wait till their respective signals
// - users must therefore be careful to only call these as necessary.
// If run is not expected to run indefinitely RunWithShutdownChan is likely to be more appropriate.
type RunnableWithShutdownFns func(atShutdown, atTerminate func(context.Context, func()))

// RunWithShutdownFns takes a function that has both atShutdown and atTerminate callbacks
// After the callback to atShutdown is called and is complete, the main function must return.
// Similarly the callback function provided to atTerminate must return once termination is complete.
// Please note that use of the atShutdown and atTerminate callbacks will create go-routines that will wait till their respective signals
// - users must therefore be careful to only call these as necessary.
// If run is not expected to run indefinitely RunWithShutdownChan is likely to be more appropriate.
func (g *gracefulManager) RunWithShutdownFns(run RunnableWithShutdownFns) {
	g.runningServerWaitGroup.Add(1)
	defer g.runningServerWaitGroup.Done()
	run(func(ctx context.Context, atShutdown func()) {
		go func() {
			select {
			case <-g.IsShutdown():
				atShutdown()
			case <-ctx.Done():
				return
			}
		}()
	}, func(ctx context.Context, atTerminate func()) {
		g.RunAtTerminate(ctx, atTerminate)
	})
}

// RunnableWithShutdownChan is a runnable with functions to run at shutdown and terminate.
// After the atShutdown channel is closed, the main function must return once shutdown is complete.
// (Optionally IsHammer may be waited for instead however, this should be avoided if possible.)
// The callback function provided to atTerminate must return once termination is complete.
// Please note that use of the atTerminate function will create a go-routine that will wait till terminate - users must therefore be careful to only call this as necessary.
type RunnableWithShutdownChan func(atShutdown <-chan struct{}, atTerminate CallbackWithContext)

// RunWithShutdownChan takes a function that has channel to watch for shutdown and atTerminate callbacks
// After the atShutdown channel is closed, the main function must return once shutdown is complete.
// (Optionally IsHammer may be waited for instead however, this should be avoided if possible.)
// The callback function provided to atTerminate must return once termination is complete.
// Please note that use of the atTerminate function will create a go-routine that will wait till terminate - users must therefore be careful to only call this as necessary.
func (g *gracefulManager) RunWithShutdownChan(run RunnableWithShutdownChan) {
	g.runningServerWaitGroup.Add(1)
	defer g.runningServerWaitGroup.Done()
	run(g.IsShutdown(), func(ctx context.Context, atTerminate func()) {
		g.RunAtTerminate(ctx, atTerminate)
	})
}

// RunWithShutdownContext takes a function that has a context to watch for shutdown.
// After the provided context is Done(), the main function must return once shutdown is complete.
// (Optionally the HammerContext may be obtained and waited for however, this should be avoided if possible.)
func (g *gracefulManager) RunWithShutdownContext(run func(context.Context)) {
	g.runningServerWaitGroup.Add(1)
	defer g.runningServerWaitGroup.Done()
	run(g.ShutdownContext())
}

// RunAtTerminate adds to the terminate wait group and creates a go-routine to run the provided function at termination
func (g *gracefulManager) RunAtTerminate(ctx context.Context, terminate func()) {
	g.terminateWaitGroup.Add(1)
	go func() {
		select {
		case <-g.IsTerminate():
			terminate()
		case <-ctx.Done():
		}
		g.terminateWaitGroup.Done()
	}()
}

// RunAtShutdown creates a go-routine to run the provided function at shutdown
func (g *gracefulManager) RunAtShutdown(ctx context.Context, shutdown func()) {
	go func() {
		select {
		case <-g.IsShutdown():
			shutdown()
		case <-ctx.Done():
		}
	}()
}

// RunAtHammer creates a go-routine to run the provided function at shutdown
func (g *gracefulManager) RunAtHammer(ctx context.Context, hammer func()) {
	go func() {
		select {
		case <-g.IsHammer():
			hammer()
		case <-ctx.Done():
		}
	}()
}
func (g *gracefulManager) doShutdown() {
	if !g.setStateTransition(stateRunning, stateShuttingDown) {
		return
	}
	g.lock.Lock()
	close(g.shutdown)
	g.lock.Unlock()

	if setting.GracefulHammerTime >= 0 {
		go g.doHammerTime(setting.GracefulHammerTime)
	}
	go func() {
		g.WaitForServers()
		// Mop up any remaining unclosed events.
		g.doHammerTime(0)
		<-time.After(1 * time.Second)
		g.doTerminate()
	}()
}

func (g *gracefulManager) doHammerTime(d time.Duration) {
	time.Sleep(d)
	select {
	case <-g.hammer:
	default:
		log.Warn("Setting Hammer condition")
		close(g.hammer)
	}

}

func (g *gracefulManager) doTerminate() {
	if !g.setStateTransition(stateShuttingDown, stateTerminate) {
		return
	}
	g.lock.Lock()
	close(g.terminate)
	g.lock.Unlock()
}

// IsChild returns if the current process is a child of previous Gitea process
func (g *gracefulManager) IsChild() bool {
	return g.isChild
}

// IsShutdown returns a channel which will be closed at shutdown.
// The order of closure is IsShutdown, IsHammer (potentially), IsTerminate
func (g *gracefulManager) IsShutdown() <-chan struct{} {
	g.lock.RLock()
	if g.shutdown == nil {
		g.lock.RUnlock()
		g.lock.Lock()
		if g.shutdown == nil {
			g.shutdown = make(chan struct{})
		}
		defer g.lock.Unlock()
		return g.shutdown
	}
	defer g.lock.RUnlock()
	return g.shutdown
}

// IsHammer returns a channel which will be closed at hammer
// The order of closure is IsShutdown, IsHammer (potentially), IsTerminate
// Servers running within the running server wait group should respond to IsHammer
// if not shutdown already
func (g *gracefulManager) IsHammer() <-chan struct{} {
	g.lock.RLock()
	if g.hammer == nil {
		g.lock.RUnlock()
		g.lock.Lock()
		if g.hammer == nil {
			g.hammer = make(chan struct{})
		}
		defer g.lock.Unlock()
		return g.hammer
	}
	defer g.lock.RUnlock()
	return g.hammer
}

// IsTerminate returns a channel which will be closed at terminate
// The order of closure is IsShutdown, IsHammer (potentially), IsTerminate
// IsTerminate will only close once all running servers have stopped
func (g *gracefulManager) IsTerminate() <-chan struct{} {
	g.lock.RLock()
	if g.terminate == nil {
		g.lock.RUnlock()
		g.lock.Lock()
		if g.terminate == nil {
			g.terminate = make(chan struct{})
		}
		defer g.lock.Unlock()
		return g.terminate
	}
	defer g.lock.RUnlock()
	return g.terminate
}

// ServerDone declares a running server done and subtracts one from the
// running server wait group. Users probably do not want to call this
// and should use one of the RunWithShutdown* functions
func (g *gracefulManager) ServerDone() {
	g.runningServerWaitGroup.Done()
}

// WaitForServers waits for all running servers to finish. Users should probably
// instead use AtTerminate or IsTerminate
func (g *gracefulManager) WaitForServers() {
	g.runningServerWaitGroup.Wait()
}

// WaitForTerminate waits for all terminating actions to finish.
// Only the main go-routine should use this
func (g *gracefulManager) WaitForTerminate() {
	g.terminateWaitGroup.Wait()
}

func (g *gracefulManager) getState() state {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.state
}

func (g *gracefulManager) setStateTransition(old, new state) bool {
	if old != g.getState() {
		return false
	}
	g.lock.Lock()
	if g.state != old {
		g.lock.Unlock()
		return false
	}
	g.state = new
	g.lock.Unlock()
	return true
}

func (g *gracefulManager) setState(st state) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.state = st
}

// InformCleanup tells the cleanup wait group that we have either taken a listener
// or will not be taking a listener
func (g *gracefulManager) InformCleanup() {
	g.createServerWaitGroup.Done()
}
