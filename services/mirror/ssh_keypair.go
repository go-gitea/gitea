// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	ssh_module "code.gitea.io/gitea/modules/ssh"
)

// GetOrCreateSSHKeypairForUser gets or creates an SSH keypair for the given user
func GetOrCreateSSHKeypairForUser(ctx context.Context, userID int64) (*repo_model.UserSSHKeypair, error) {
	return ssh_module.GetOrCreateSSHKeypairForUser(ctx, userID)
}

// GetOrCreateSSHKeypairForOrg gets or creates an SSH keypair for the given organization
func GetOrCreateSSHKeypairForOrg(ctx context.Context, orgID int64) (*repo_model.UserSSHKeypair, error) {
	return ssh_module.GetOrCreateSSHKeypairForOrg(ctx, orgID)
}

// GetSSHKeypairForRepository gets the appropriate SSH keypair for a repository
// If the repository belongs to an organization, it uses the org's keypair,
// otherwise it uses the user's keypair
func GetSSHKeypairForRepository(ctx context.Context, repo *repo_model.Repository) (*repo_model.UserSSHKeypair, error) {
	return ssh_module.GetSSHKeypairForRepository(ctx, repo)
}

// RegenerateSSHKeypairForUser regenerates the SSH keypair for a user
func RegenerateSSHKeypairForUser(ctx context.Context, userID int64) (*repo_model.UserSSHKeypair, error) {
	log.Info("Regenerating SSH keypair for user %d", userID)
	return repo_model.RegenerateUserSSHKeypair(ctx, userID)
}

// RegenerateSSHKeypairForOrg regenerates the SSH keypair for an organization
func RegenerateSSHKeypairForOrg(ctx context.Context, orgID int64) (*repo_model.UserSSHKeypair, error) {
	log.Info("Regenerating SSH keypair for organization %d", orgID)
	return repo_model.RegenerateUserSSHKeypair(ctx, orgID)
}

// IsSSHURL checks if a URL is an SSH URL
func IsSSHURL(url string) bool {
	return ssh_module.IsSSHURL(url)
}

// GetSSHKeypairForURL gets the appropriate SSH keypair for a given repository and URL
// Returns nil if the URL is not an SSH URL
func GetSSHKeypairForURL(ctx context.Context, repo *repo_model.Repository, url string) (*repo_model.UserSSHKeypair, error) {
	return ssh_module.GetSSHKeypairForURL(ctx, repo, url)
}
