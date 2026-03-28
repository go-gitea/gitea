// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package ssh

import (
	"fmt"
	"net"

	"code.gitea.io/gitea/modules/util"

	"github.com/Microsoft/go-winio"
)

// createAgentListener creates a Windows named pipe listener for the SSH agent.
// Returns the listener, pipe path, and a cleanup function for early-return error paths.
func createAgentListener() (net.Listener, string, func(), error) {
	agentID, err := util.CryptoRandomString(16)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate agent ID: %w", err)
	}
	pipePath := `\\.\pipe\gitea-ssh-agent-` + agentID

	listener, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create named pipe: %w", err)
	}

	cleanup := func() {
		listener.Close()
	}

	return listener, pipePath, cleanup, nil
}

// setListenerAcceptDeadline is a no-op on Windows; named pipes don't support SetDeadline.
func setListenerAcceptDeadline(_ net.Listener) {}

// cleanupAgentSocket is a no-op on Windows; named pipes are automatically cleaned up when closed.
func cleanupAgentSocket(_ string) {}
