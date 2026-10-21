// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"fmt"
	"os"

	user_model "gitea.dev/models/user"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/setting"
)

// SetupManagedSSHAgent returns the environment for a git command reaching remoteURL with keyOwnerID's managed SSH key.
// Non-SSH remotes get nil, which keeps the process environment. cleanup is never nil.
func SetupManagedSSHAgent(ctx context.Context, keyOwnerID int64, remoteURL string) (envs []string, cleanup func(), err error) {
	noop := func() {}
	if !giturl.IsSSH(remoteURL) {
		return nil, noop, nil
	}

	keypair, err := user_model.GetOrCreateSSHKeypair(ctx, keyOwnerID)
	if err != nil {
		return nil, noop, err
	}
	privateKey, err := keypair.GetDecryptedPrivateKey()
	if err != nil {
		return nil, noop, err
	}

	socketPath, agentCleanup, err := startAgent(privateKey)
	if err != nil {
		return nil, noop, fmt.Errorf("failed to create SSH agent: %w", err)
	}
	identityFile, keyCleanup, err := writeManagedPublicKey(keypair.PublicKey)
	if err != nil {
		agentCleanup()
		return nil, noop, fmt.Errorf("failed to write managed public key: %w", err)
	}

	// appended last so they win over SSH_AUTH_SOCK or GIT_SSH_COMMAND inherited from the process
	envs = append(os.Environ(), "SSH_AUTH_SOCK="+socketPath, "GIT_SSH_COMMAND="+managedSSHCommand(identityFile))
	return envs, func() {
		keyCleanup()
		agentCleanup()
	}, nil
}

// writeManagedPublicKey lets GIT_SSH_COMMAND pin authentication to the managed key via "-i"
func writeManagedPublicKey(publicKey string) (path string, cleanup func(), err error) {
	f, cleanup, err := setting.AppDataTempDir("ssh-agent").CreateTempFileRandom("gitea-managed-ssh-*.pub")
	if err != nil {
		return "", nil, err
	}
	if _, err = f.WriteString(publicKey); err != nil {
		cleanup()
		return "", nil, err
	}
	if err = f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return f.Name(), cleanup, nil
}
