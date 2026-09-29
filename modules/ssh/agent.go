// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"sync"

	"gitea.dev/modules/log"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Agent represents a temporary SSH agent for repo mirroring
type Agent struct {
	socketPath string
	listener   net.Listener
	agent      agent.Agent
	stop       chan struct{}
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
}

// NewSSHAgent creates a new SSH agent with the given private key
func NewSSHAgent(privateKey ed25519.PrivateKey) (*Agent, error) {
	listener, socketPath, cleanup, err := createAgentListener()
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	sshAgent := agent.NewKeyring()

	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	_, err = ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH signer: %w", err)
	}

	err = sshAgent.Add(agent.AddedKey{
		PrivateKey: privateKey,
		Comment:    "gitea-mirror-key",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add key to agent: %w", err)
	}

	// Create our SSH agent wrapper
	sa := &Agent{
		socketPath: socketPath,
		listener:   listener,
		agent:      sshAgent,
		stop:       make(chan struct{}),
	}

	// Start serving
	sa.wg.Add(1)
	go sa.serve()

	// Clear cleanup since we're returning successfully
	cleanup = nil

	return sa, nil
}

// serve handles incoming connections to the SSH agent
func (sa *Agent) serve() {
	defer sa.wg.Done()
	defer sa.cleanup()

	for {
		// Close() closes sa.stop then the listener, which unblocks Accept here.
		conn, err := sa.listener.Accept()
		if err != nil {
			select {
			case <-sa.stop:
				return
			default:
				log.Error("SSH agent failed to accept connection: %v", err)
				continue
			}
		}

		sa.wg.Add(1)
		go func(c net.Conn) {
			defer sa.wg.Done()
			defer c.Close()

			// ServeAgent only returns once the connection ends, always with a non-nil error.
			err := agent.ServeAgent(sa.agent, c)
			log.Debug("SSH agent connection ended: %v", err)
		}(conn)
	}
}

// cleanup removes the socket file and temporary directory
func (sa *Agent) cleanup() {
	cleanupAgentSocket(sa.socketPath)
}

// GetSocketPath returns the path to the SSH agent socket
func (sa *Agent) GetSocketPath() string {
	return sa.socketPath
}

// Close stops the SSH agent and cleans up resources
func (sa *Agent) Close() error {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if sa.closed {
		return nil
	}
	sa.closed = true

	close(sa.stop)

	if sa.listener != nil {
		sa.listener.Close()
	}

	sa.wg.Wait()

	return nil
}

// CreateTemporaryAgent creates a temporary SSH agent with the given private key.
// It returns the socket path for use with SSH_AUTH_SOCK and a cleanup function
// that the caller must invoke (typically via defer) once the git operation is done.
func CreateTemporaryAgent(privateKey ed25519.PrivateKey) (string, func(), error) {
	agent, err := NewSSHAgent(privateKey)
	if err != nil {
		return "", nil, err
	}
	return agent.GetSocketPath(), func() { _ = agent.Close() }, nil
}
