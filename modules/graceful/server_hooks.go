// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"os"

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
	if srv.getState() != stateShuttingDown {
		return
	}
	srv.closeAllConnections()
}
