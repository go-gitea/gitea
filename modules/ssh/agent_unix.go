// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gitea.dev/modules/setting"
)

// createAgentListener listens on a Unix socket only the current user can reach, cleanup closes it and removes its directory
func createAgentListener() (net.Listener, string, func(), error) {
	tempDir, cleanupDir, err := setting.AppDataTempDir("ssh-agent").MkdirTempRandom("gitea-ssh-agent-")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	if err := os.Chmod(tempDir, 0o700); err != nil {
		cleanupDir()
		return nil, "", nil, fmt.Errorf("failed to set temporary directory permissions: %w", err)
	}

	socketPath := filepath.Join(tempDir, "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		cleanupDir()
		return nil, "", nil, fmt.Errorf("failed to create Unix socket: %w", err)
	}

	cleanup := func() {
		listener.Close()
		cleanupDir()
	}

	if err := os.Chmod(socketPath, 0o600); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, socketPath, cleanup, nil
}
