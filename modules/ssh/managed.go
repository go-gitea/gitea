// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
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
		if errors.Is(err, util.ErrNotExist) {
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
// If sshKeyOwnerID is non-zero, the keypair of that owner is used instead of
// the repository owner's (used when migrating to an org and the user wants
// to authenticate with their personal managed key).
func SetupManagedSSHAgent(ctx context.Context, repo *repo_model.Repository, remoteURL string, sshKeyOwnerID int64) (sshAuth gitcmd.SSHAuth, cleanup func(), err error) {
	noop := func() {}
	if !IsSSHURL(remoteURL) {
		return gitcmd.SSHAuth{}, noop, nil
	}

	ownerID := repo.OwnerID
	if sshKeyOwnerID != 0 {
		ownerID = sshKeyOwnerID
	}
	keypair, err := GetOrCreateSSHKeypair(ctx, ownerID)
	if err != nil {
		return gitcmd.SSHAuth{}, noop, fmt.Errorf("failed to get SSH keypair for owner %d: %w", ownerID, err)
	}
	if keypair == nil {
		return gitcmd.SSHAuth{}, noop, nil
	}

	privateKey, err := keypair.GetDecryptedPrivateKey()
	if err != nil {
		return gitcmd.SSHAuth{}, noop, fmt.Errorf("failed to decrypt SSH private key: %w", err)
	}

	socketPath, agentCleanup, err := CreateTemporaryAgent(privateKey)
	if err != nil {
		return gitcmd.SSHAuth{}, noop, fmt.Errorf("failed to create SSH agent: %w", err)
	}

	identityFile, keyCleanup, err := writeManagedPublicKey(keypair.PublicKey)
	if err != nil {
		agentCleanup()
		return gitcmd.SSHAuth{}, noop, fmt.Errorf("failed to write managed public key: %w", err)
	}

	cleanup = func() {
		keyCleanup()
		agentCleanup()
	}

	log.Debug("SSH agent ready for %s (socket: %s)", repo.FullName(), socketPath)
	return gitcmd.SSHAuth{AuthSock: socketPath, IdentityFile: identityFile}, cleanup, nil
}

// writeManagedPublicKey writes the managed public key to a temporary file so the
// git SSH command can pin authentication to it via "-i". The returned cleanup
// removes the file.
func writeManagedPublicKey(publicKey string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "gitea-managed-ssh-*.pub")
	if err != nil {
		return "", nil, err
	}
	if _, err = f.WriteString(publicKey); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err = f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
