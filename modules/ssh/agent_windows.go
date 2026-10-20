// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package ssh

import (
	"fmt"
	"net"

	"gitea.dev/modules/util"

	"github.com/Microsoft/go-winio"
)

// createAgentListener listens on a named pipe, which is removed once closed
func createAgentListener() (net.Listener, string, func(), error) {
	agentID := util.CryptoRandomString(16)
	pipePath := `\\.\pipe\gitea-ssh-agent-` + agentID

	listener, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create named pipe: %w", err)
	}

	return listener, pipePath, func() { listener.Close() }, nil
}
