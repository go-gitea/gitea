// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHAgent represents a temporary SSH agent for repo mirroring
type SSHAgent struct {
	socketPath string
	listener   net.Listener
	agent      agent.Agent
	stop       chan struct{}
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
}

// NewSSHAgent creates a new SSH agent with the given private key
func NewSSHAgent(privateKey ed25519.PrivateKey) (*SSHAgent, error) {
	var listener net.Listener
	var socketPath string
	var tempDir string
	var err error

	// Setup cleanup function for early returns
	var cleanup func()
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	if runtime.GOOS == "windows" {
		// On Windows, use named pipes
		agentID, err := util.CryptoRandomString(16)
		if err != nil {
			return nil, fmt.Errorf("failed to generate agent ID: %w", err)
		}
		socketPath = `\\.\pipe\gitea-ssh-agent-` + agentID
		listener, err = net.Listen("pipe", socketPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create named pipe: %w", err)
		}
		cleanup = func() {
			listener.Close()
		}
	} else {
		tempDir, err = os.MkdirTemp("", "gitea-ssh-agent-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory: %w", err)
		}
		cleanup = func() {
			os.RemoveAll(tempDir)
		}

		if err := os.Chmod(tempDir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to set temporary directory permissions: %w", err)
		}

		socketPath = filepath.Join(tempDir, "agent.sock")
		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create Unix socket: %w", err)
		}
		cleanup = func() {
			listener.Close()
			os.RemoveAll(tempDir)
		}

		if err := os.Chmod(socketPath, 0o600); err != nil {
			return nil, fmt.Errorf("failed to set socket permissions: %w", err)
		}
	}

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
	sa := &SSHAgent{
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
func (sa *SSHAgent) serve() {
	defer sa.wg.Done()
	defer sa.cleanup()

	for {
		select {
		case <-sa.stop:
			return
		default:
			// Set a timeout for Accept to avoid blocking indefinitely
			if runtime.GOOS != "windows" {
				// On Windows, named pipes don't support SetDeadline in the same way
				if listener, ok := sa.listener.(*net.UnixListener); ok {
					listener.SetDeadline(time.Now().Add(100 * time.Millisecond))
				}
			}

			conn, err := sa.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
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

				err := agent.ServeAgent(sa.agent, c)
				if err != nil {
					log.Debug("SSH agent connection ended: %v", err)
				}
			}(conn)
		}
	}
}

// cleanup removes the socket file and temporary directory
func (sa *SSHAgent) cleanup() {
	if sa.socketPath != "" {
		if runtime.GOOS != "windows" {
			// On Windows, named pipes are automatically cleaned up when closed
			// On Unix-like systems, remove the temporary directory
			tempDir := filepath.Dir(sa.socketPath)
			os.RemoveAll(tempDir)
		}
	}
}

// GetSocketPath returns the path to the SSH agent socket
func (sa *SSHAgent) GetSocketPath() string {
	return sa.socketPath
}

// Close stops the SSH agent and cleans up resources
func (sa *SSHAgent) Close() error {
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

// SSHAgentManager manages temporary SSH agents for git operations
type SSHAgentManager struct {
	mu     sync.Mutex
	agents map[string]*SSHAgent
}

var globalAgentManager = &SSHAgentManager{
	agents: make(map[string]*SSHAgent),
}

// CreateTemporaryAgent creates a temporary SSH agent with the given private key
// Returns the socket path for use with SSH_AUTH_SOCK
func CreateTemporaryAgent(privateKey ed25519.PrivateKey) (string, func(), error) {
	agent, err := NewSSHAgent(privateKey)
	if err != nil {
		return "", nil, err
	}

	agentID, err := util.CryptoRandomString(16)
	if err != nil {
		agent.Close()
		return "", nil, fmt.Errorf("failed to generate agent ID: %w", err)
	}

	globalAgentManager.mu.Lock()
	globalAgentManager.agents[agentID] = agent
	globalAgentManager.mu.Unlock()

	cleanup := func() {
		globalAgentManager.mu.Lock()
		defer globalAgentManager.mu.Unlock()

		if agent, exists := globalAgentManager.agents[agentID]; exists {
			agent.Close()
			delete(globalAgentManager.agents, agentID)
		}
	}

	return agent.GetSocketPath(), cleanup, nil
}

// CleanupAllAgents closes all active SSH agents (should be called on shutdown)
func CleanupAllAgents() {
	globalAgentManager.mu.Lock()
	defer globalAgentManager.mu.Unlock()

	for id, agent := range globalAgentManager.agents {
		agent.Close()
		delete(globalAgentManager.agents, id)
	}
}
