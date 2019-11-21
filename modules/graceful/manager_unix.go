// +build !windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type gracefulManager struct {
	isChild                bool
	forked                 bool
	lock                   *sync.RWMutex
	state                  state
	shutdown               chan struct{}
	hammer                 chan struct{}
	terminate              chan struct{}
	runningServerWaitGroup sync.WaitGroup
	createServerWaitGroup  sync.WaitGroup
	terminateWaitGroup     sync.WaitGroup
}

func newGracefulManager() *gracefulManager {
	manager := &gracefulManager{
		isChild: len(os.Getenv(listenFDs)) > 0 && os.Getppid() > 1,
		lock:    &sync.RWMutex{},
	}
	manager.createServerWaitGroup.Add(numberOfServersToCreate)
	manager.Run()
	return manager
}

func (g *gracefulManager) Run() {
	g.setState(stateRunning)
	go g.handleSignals()
	c := make(chan struct{})
	go func() {
		defer close(c)
		// Wait till we're done getting all of the listeners and then close
		// the unused ones
		g.createServerWaitGroup.Wait()
		// Ignore the error here there's not much we can do with it
		// They're logged in the CloseProvidedListeners function
		_ = CloseProvidedListeners()
	}()
	if setting.StartupTimeout > 0 {
		go func() {
			select {
			case <-c:
				return
			case <-g.IsShutdown():
				return
			case <-time.After(setting.StartupTimeout):
				log.Error("Startup took too long! Shutting down")
				g.doShutdown()
			}
		}()
	}
}

func (g *gracefulManager) handleSignals() {
	var sig os.Signal

	signalChannel := make(chan os.Signal, 1)

	signal.Notify(
		signalChannel,
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGTSTP,
	)

	pid := syscall.Getpid()
	for {
		sig = <-signalChannel
		switch sig {
		case syscall.SIGHUP:
			if setting.GracefulRestartable {
				log.Info("PID: %d. Received SIGHUP. Forking...", pid)
				err := g.doFork()
				if err != nil && err.Error() != "another process already forked. Ignoring this one" {
					log.Error("Error whilst forking from PID: %d : %v", pid, err)
				}
			} else {
				log.Info("PID: %d. Received SIGHUP. Not set restartable. Shutting down...", pid)

				g.doShutdown()
			}
		case syscall.SIGUSR1:
			log.Info("PID %d. Received SIGUSR1.", pid)
		case syscall.SIGUSR2:
			log.Warn("PID %d. Received SIGUSR2. Hammering...", pid)
			g.doHammerTime(0 * time.Second)
		case syscall.SIGINT:
			log.Warn("PID %d. Received SIGINT. Shutting down...", pid)
			g.doShutdown()
		case syscall.SIGTERM:
			log.Warn("PID %d. Received SIGTERM. Shutting down...", pid)
			g.doShutdown()
		case syscall.SIGTSTP:
			log.Info("PID %d. Received SIGTSTP.", pid)
		default:
			log.Info("PID %d. Received %v.", pid, sig)
		}
	}
}

func (g *gracefulManager) doFork() error {
	g.lock.Lock()
	if g.forked {
		g.lock.Unlock()
		return errors.New("another process already forked. Ignoring this one")
	}
	g.forked = true
	g.lock.Unlock()
	// We need to move the file logs to append pids
	setting.RestartLogsWithPIDSuffix()

	_, err := RestartProcess()

	return err
}

func (g *gracefulManager) RegisterServer() {
	KillParent()
	g.runningServerWaitGroup.Add(1)
}
