// +build windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"code.gitea.io/gitea/modules/log"
	"github.com/gliderlabs/ssh"
)

func listen(server *ssh.Server) {
	err := server.ListenAndServe()
	if err != nil {
		log.Critical("Failed to serve with builtin SSH server. %s", err)
	}
}

// Unused does nothing on windows
func Unused() {
	// Do nothing
}
