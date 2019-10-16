// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var hookableSignals []os.Signal

func init() {
	hookableSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGTSTP,
	}
}

// handleSignals listens for os Signals and calls any hooked in function that the
// user had registered with the signal.
func (srv *Server) handleSignals() {
	var sig os.Signal

	signal.Notify(
		srv.sigChan,
		hookableSignals...,
	)

	pid := syscall.Getpid()
	for {
		sig = <-srv.sigChan
		srv.preSignalHooks(sig)
		switch sig {
		case syscall.SIGHUP:
			if setting.GracefulRestartable {
				log.Info("PID: %d. Received SIGHUP. Forking...", pid)
				err := srv.fork()
				if err != nil {
					log.Error("Error whilst forking from PID: %d : %v", pid, err)
				}
			} else {
				log.Info("PID: %d. Received SIGHUP. Not set restartable. Shutting down...", pid)

				srv.shutdown()
			}
		case syscall.SIGUSR1:
			log.Info("PID %d. Received SIGUSR1.", pid)
		case syscall.SIGUSR2:
			log.Warn("PID %d. Received SIGUSR2. Hammering...", pid)
			srv.hammerTime(0 * time.Second)
		case syscall.SIGINT:
			log.Warn("PID %d. Received SIGINT. Shutting down...", pid)
			srv.shutdown()
		case syscall.SIGTERM:
			log.Warn("PID %d. Received SIGTERM. Shutting down...", pid)
			srv.shutdown()
		case syscall.SIGTSTP:
			log.Info("PID %d. Received SIGTSTP.")
		default:
			log.Info("PID %d. Received %v.", sig)
		}
		srv.postSignalHooks(sig)
	}
}

func (srv *Server) preSignalHooks(sig os.Signal) {
	if _, notSet := srv.PreSignalHooks[sig]; !notSet {
		return
	}
	for _, f := range srv.PreSignalHooks[sig] {
		f()
	}
}

func (srv *Server) postSignalHooks(sig os.Signal) {
	if _, notSet := srv.PostSignalHooks[sig]; !notSet {
		return
	}
	for _, f := range srv.PostSignalHooks[sig] {
		f()
	}
}
