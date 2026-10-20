// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ed25519"
	"fmt"
	"sync"

	"gitea.dev/modules/log"

	"golang.org/x/crypto/ssh/agent"
)

// startAgent serves privateKey on a fresh agent socket for SSH_AUTH_SOCK, cleanup stops the agent and removes the socket
func startAgent(privateKey ed25519.PrivateKey) (socketPath string, cleanup func(), err error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", nil, fmt.Errorf("invalid Ed25519 private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: privateKey, Comment: "gitea-managed-key"}); err != nil {
		return "", nil, fmt.Errorf("failed to add key to agent: %w", err)
	}

	listener, socketPath, closeListener, err := createAgentListener()
	if err != nil {
		return "", nil, err
	}

	stopping := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stopping:
					return
				default:
					log.Error("SSH agent failed to accept connection: %v", err)
					continue
				}
			}
			wg.Go(func() {
				defer conn.Close()
				// ServeAgent only returns once the connection ends, always with a non-nil error
				log.Debug("SSH agent connection ended: %v", agent.ServeAgent(keyring, conn))
			})
		}
	})

	// mirror sync may run the deferred cleanup after an explicit one
	return socketPath, sync.OnceFunc(func() {
		close(stopping)
		closeListener()
		wg.Wait()
	}), nil
}
