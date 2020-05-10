// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"os"
	"runtime"

	"code.gitea.io/gitea/modules/log"
)

// awaitShutdown waits for the shutdown signal from the Manager
func (srv *Server) awaitShutdown() {
	select {
	case <-GetManager().IsShutdown():
		// Shutdown
		srv.doShutdown()
	case <-GetManager().IsHammer():
		// Hammer
		srv.doShutdown()
		srv.doHammer()
	}
	<-GetManager().IsHammer()
	srv.doHammer()
}

// shutdown closes the listener so that no new connections are accepted
// and starts a goroutine that will hammer (stop all running requests) the server
// after setting.GracefulHammerTime.
func (srv *Server) doShutdown() {
	// only shutdown if we're running.
	if srv.getState() != stateRunning {
		return
	}

	srv.setState(stateShuttingDown)

	if srv.OnShutdown != nil {
		srv.OnShutdown()
	}
	err := srv.listener.Close()
	if err != nil {
		log.Error("PID: %d Listener.Close() error: %v", os.Getpid(), err)
	} else {
		log.Info("PID: %d Listener (%s) closed.", os.Getpid(), srv.listener.Addr())
	}
}

func (srv *Server) doHammer() {
	defer func() {
		// We call srv.wg.Done() until it panics.
		// This happens if we call Done() when the WaitGroup counter is already at 0
		// So if it panics -> we're done, Serve() will return and the
		// parent will goroutine will exit.
		if r := recover(); r != nil {
			log.Error("WaitGroup at 0: Error: %v", r)
		}
	}()
	if srv.getState() != stateShuttingDown {
		return
	}
	log.Warn("Forcefully shutting down parent")
	for {
		if srv.getState() == stateTerminate {
			break
		}
		srv.wg.Done()

		// Give other goroutines a chance to finish before we forcibly stop them.
		runtime.Gosched()
	}
}
