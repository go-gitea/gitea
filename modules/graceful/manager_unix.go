// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package graceful

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"code.gitea.io/gitea/modules/graceful/releasereopen"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

func pidMsg() systemdNotifyMsg {
	return systemdNotifyMsg("MAINPID=" + strconv.Itoa(os.Getpid()))
}

// Notify systemd of status via the notify protocol
func (g *Manager) notify(msg systemdNotifyMsg) {
	conn, err := getNotifySocket()
	if err != nil {
		// the err is logged in getNotifySocket
		return
	}
	if conn == nil {
		return
	}
	defer conn.Close()

	if _, err = conn.Write([]byte(msg)); err != nil {
		log.Warn("Failed to notify NOTIFY_SOCKET: %v", err)
		return
	}
}

func (g *Manager) start() {
	// Now label this and all goroutines created by this goroutine with the graceful-lifecycle manager
	pprof.SetGoroutineLabels(g.managerCtx)
	defer pprof.SetGoroutineLabels(g.ctx)

	g.isChild = len(os.Getenv(listenFDsEnv)) > 0 && os.Getppid() > 1

	g.notify(statusMsg("Starting Gitea"))
	g.notify(pidMsg())
	go g.handleSignals(g.managerCtx)

	// Handle clean up of unused provided listeners	and delayed start-up
	startupDone := make(chan struct{})
	go func() {
		defer func() {
			close(startupDone)
			// Close the unused listeners and ignore the error here there's not much we can do with it, they're logged in the CloseProvidedListeners function
			_ = CloseProvidedListeners()
		}()
		// Wait for all servers to be created
		g.createServerCond.L.Lock()
		for {
			if g.createdServer >= numberOfServersToCreate {
				g.createServerCond.L.Unlock()
				g.notify(readyMsg)
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
	if setting.StartupTimeout > 0 {
		go func() {
			select {
			case <-startupDone:
				return
			case <-g.IsShutdown():
				g.createServerCond.Signal()
				return
			case <-time.After(setting.StartupTimeout):
				log.Error("Startup took too long! Shutting down")
				g.notify(statusMsg("Startup took too long! Shutting down"))
				g.notify(stoppingMsg)
				g.doShutdown()
			}
		}()
	}
}

func (g *Manager) handleSignals(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Graceful: HandleSignals", process.SystemProcessType, true)
	defer finished()

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

	watchdogTimeout := getWatchdogTimeout()
	t := &time.Ticker{}
	if watchdogTimeout != 0 {
		g.notify(watchdogMsg)
		t = time.NewTicker(watchdogTimeout / 2)
	}

	pid := syscall.Getpid()
	for {
		select {
		case sig := <-signalChannel:
			switch sig {
			case syscall.SIGHUP:
				log.Info("PID: %d. Received SIGHUP. Attempting GracefulRestart...", pid)
				g.DoGracefulRestart()
			case syscall.SIGUSR1:
				log.Warn("PID %d. Received SIGUSR1. Releasing and reopening logs", pid)
				g.notify(statusMsg("Releasing and reopening logs"))
				if err := releasereopen.GetManager().ReleaseReopen(); err != nil {
					log.Error("Error whilst releasing and reopening logs: %v", err)
				}
			case syscall.SIGUSR2:
				log.Warn("PID %d. Received SIGUSR2. Hammering...", pid)
				g.DoImmediateHammer()
			case syscall.SIGINT:
				log.Warn("PID %d. Received SIGINT. Shutting down...", pid)
				g.DoGracefulShutdown()
			case syscall.SIGTERM:
				log.Warn("PID %d. Received SIGTERM. Shutting down...", pid)
				g.DoGracefulShutdown()
			case syscall.SIGTSTP:
				log.Info("PID %d. Received SIGTSTP.", pid)
			default:
				log.Info("PID %d. Received %v.", pid, sig)
			}
		case <-t.C:
			g.notify(watchdogMsg)
		case <-ctx.Done():
			log.Warn("PID: %d. Background context for manager closed - %v - Shutting down...", pid, ctx.Err())
			g.DoGracefulShutdown()
			return
		}
	}
}

func (g *Manager) doFork() error {
	g.lock.Lock()
	if g.forked {
		g.lock.Unlock()
		return errors.New("another process already forked. Ignoring this one")
	}
	g.forked = true
	g.lock.Unlock()

	g.notify(reloadingMsg)

	// We need to move the file logs to append pids
	setting.RestartLogsWithPIDSuffix()

	_, err := RestartProcess()

	return err
}

// DoGracefulRestart causes a graceful restart
func (g *Manager) DoGracefulRestart() {
	if setting.GracefulRestartable {
		log.Info("PID: %d. Forking...", os.Getpid())
		err := g.doFork()
		if err != nil {
			if err.Error() == "another process already forked. Ignoring this one" {
				g.DoImmediateHammer()
			} else {
				log.Error("Error whilst forking from PID: %d : %v", os.Getpid(), err)
			}
		}
		// doFork calls RestartProcess which starts a new Gitea process, so this parent process needs to exit
		// Otherwise some resources (eg: leveldb lock) will be held by this parent process and the new process will fail to start
		log.Info("PID: %d. Shutting down after forking ...", os.Getpid())
		g.doShutdown()
	} else {
		log.Info("PID: %d. Not set restartable. Shutting down...", os.Getpid())
		g.notify(stoppingMsg)
		g.doShutdown()
	}
}
