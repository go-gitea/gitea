// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"time"

	"code.gitea.io/gitea/modules/log"
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
	Manager = newGracefulManager()
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
