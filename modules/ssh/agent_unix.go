// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// createAgentListener creates a Unix domain socket listener for the SSH agent.
// Returns the listener, socket path, and a cleanup function for early-return error paths.
func createAgentListener() (net.Listener, string, func(), error) {
	tempDir, err := os.MkdirTemp("", "gitea-ssh-agent-")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	cleanupDir := func() {
		os.RemoveAll(tempDir)
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
		os.RemoveAll(tempDir)
	}

	if err := os.Chmod(socketPath, 0o600); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, socketPath, cleanup, nil
}

// setListenerAcceptDeadline sets a short deadline on the listener for non-blocking accept loops.
func setListenerAcceptDeadline(listener net.Listener) {
	if unixListener, ok := listener.(*net.UnixListener); ok {
		_ = unixListener.SetDeadline(time.Now().Add(100 * time.Millisecond))
	}
}

// cleanupAgentSocket removes the socket file and its temporary directory.
func cleanupAgentSocket(socketPath string) {
	if socketPath != "" {
		tempDir := filepath.Dir(socketPath)
		os.RemoveAll(tempDir)
	}
}
