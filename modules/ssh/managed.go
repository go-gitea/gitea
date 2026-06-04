// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/log"
)

// IsSSHURL checks if a URL is an SSH URL
func IsSSHURL(remote string) bool {
	u, err := giturl.ParseGitURL(remote)
	return err == nil && u.Scheme == "ssh"
}

// GetOrCreateSSHKeypair gets or creates the managed SSH keypair for the given
// owner (user or organization — they share the same backing storage).
func GetOrCreateSSHKeypair(ctx context.Context, ownerID int64) (*user_model.SSHKeypair, error) {
	keypair, err := user_model.GetSSHKeypairByOwner(ctx, ownerID)
	if err != nil {
		if db.IsErrNotExist(err) {
			log.Debug("Creating new SSH keypair for owner %d", ownerID)
			return user_model.CreateSSHKeypair(ctx, ownerID)
		}
		return nil, fmt.Errorf("failed to get SSH keypair for owner %d: %w", ownerID, err)
	}
	return keypair, nil
}

// GetSSHKeypairForRepository gets the managed SSH keypair for the repository's owner.
func GetSSHKeypairForRepository(ctx context.Context, repo *repo_model.Repository) (*user_model.SSHKeypair, error) {
	return GetOrCreateSSHKeypair(ctx, repo.OwnerID)
}

// SetupManagedSSHAgent prepares SSH key-based authentication for a mirror or
// migration git operation against remoteURL on behalf of repo. For non-SSH
// URLs (or when no keypair is available) it is a no-op. The returned cleanup
// is never nil and must always be called by the caller (typically via defer).
func SetupManagedSSHAgent(ctx context.Context, repo *repo_model.Repository, remoteURL string) (sshAuthSock string, cleanup func(), err error) {
	noop := func() {}
	if !IsSSHURL(remoteURL) {
		return "", noop, nil
	}

	keypair, err := GetSSHKeypairForRepository(ctx, repo)
	if err != nil {
		return "", noop, fmt.Errorf("failed to get SSH keypair for repository: %w", err)
	}
	if keypair == nil {
		return "", noop, nil
	}

	privateKey, err := keypair.GetDecryptedPrivateKey()
	if err != nil {
		return "", noop, fmt.Errorf("failed to decrypt SSH private key: %w", err)
	}

	socketPath, agentCleanup, err := CreateTemporaryAgent(privateKey)
	if err != nil {
		return "", noop, fmt.Errorf("failed to create SSH agent: %w", err)
	}

	log.Debug("SSH agent ready for mirror %s (socket: %s)", repo.FullName(), socketPath)
	return socketPath, agentCleanup, nil
}
