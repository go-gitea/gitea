// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// IsSSHURL checks if a URL is an SSH URL
func IsSSHURL(url string) bool {
	return strings.HasPrefix(url, "ssh://")
}

// GetOrCreateSSHKeypairForUser gets or creates an SSH keypair for the given user
func GetOrCreateSSHKeypairForUser(ctx context.Context, userID int64) (*repo_model.UserSSHKeypair, error) {
	keypair, err := repo_model.GetUserSSHKeypairByOwner(ctx, userID)
	if err != nil {
		if db.IsErrNotExist(err) {
			log.Debug("Creating new SSH keypair for user %d", userID)
			return repo_model.CreateUserSSHKeypair(ctx, userID)
		}
		return nil, fmt.Errorf("failed to get SSH keypair for user %d: %w", userID, err)
	}
	return keypair, nil
}

// GetOrCreateSSHKeypairForOrg gets or creates an SSH keypair for the given organization
func GetOrCreateSSHKeypairForOrg(ctx context.Context, orgID int64) (*repo_model.UserSSHKeypair, error) {
	keypair, err := repo_model.GetUserSSHKeypairByOwner(ctx, orgID)
	if err != nil {
		if db.IsErrNotExist(err) {
			log.Debug("Creating new SSH keypair for organization %d", orgID)
			return repo_model.CreateUserSSHKeypair(ctx, orgID)
		}
		return nil, fmt.Errorf("failed to get SSH keypair for organization %d: %w", orgID, err)
	}
	return keypair, nil
}

// GetSSHKeypairForRepository gets the appropriate SSH keypair for a repository
// If the repository belongs to an organization, it uses the org's keypair,
// otherwise it uses the user's keypair
func GetSSHKeypairForRepository(ctx context.Context, repo *repo_model.Repository) (*repo_model.UserSSHKeypair, error) {
	if repo.Owner == nil {
		owner, err := user_model.GetUserByID(ctx, repo.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository owner: %w", err)
		}
		repo.Owner = owner
	}

	if repo.Owner.IsOrganization() {
		return GetOrCreateSSHKeypairForOrg(ctx, repo.OwnerID)
	}
	return GetOrCreateSSHKeypairForUser(ctx, repo.OwnerID)
}

// GetSSHKeypairForURL gets the appropriate SSH keypair for a given repository and URL
// Returns nil if the URL is not an SSH URL
func GetSSHKeypairForURL(ctx context.Context, repo *repo_model.Repository, url string) (*repo_model.UserSSHKeypair, error) {
	if !IsSSHURL(url) {
		return nil, nil
	}
	return GetSSHKeypairForRepository(ctx, repo)
}
