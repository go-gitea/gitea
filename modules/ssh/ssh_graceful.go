// +build !windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"

	"github.com/gliderlabs/ssh"
)

func listen(server *ssh.Server) {
	gracefulServer := graceful.NewServer("tcp", server.Addr)

	err := gracefulServer.ListenAndServe(server.Serve)
	if err != nil {
		if strings.Contains(err.Error(), "use of closed") {
			log.Info("SSH Listener: %s Closed", server.Addr)
		} else {
			log.Critical("Failed to start SSH server: %v", err)
		}
	}
}
