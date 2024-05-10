// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
)

// Check if a user have a new pinned repo in it's profile, meaning that it
// has permissions to pin said repo and also has enough space on the pinned list.
func CanPin(ctx *context.Context, u *user_model.User, r *repo_model.Repository) bool {
	repos, err := repo_model.GetPinnedRepos(*ctx, &repo_model.PinnedReposOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		PinnerID: u.ID,
	})
	if err != nil {
		ctx.ServerError("GetPinnedRepos", err)
		return false
	}
	if len(repos) >= 6 {
		return false
	}

	return HasPermsToPin(ctx, u, r)
}

// Checks if the user has permission to have the repo pinned in it's profile.
func HasPermsToPin(ctx *context.Context, u *user_model.User, r *repo_model.Repository) bool {
	// If user is an organization, it can only pin its own repos
	if u.IsOrganization() {
		return r.OwnerID == u.ID
	}

	// For normal users, anyone that has read access to the repo can pin it
	return canSeePin(ctx, u, r)
}

// Check if a user can see a pin
// A user can see a pin if he has read access to the repo
func canSeePin(ctx *context.Context, u *user_model.User, r *repo_model.Repository) bool {
	perm, err := access_model.GetUserRepoPermission(ctx, r, u)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return false
	}
	return perm.HasAnyUnitAccess()
}

// CleanupPins iterates over the repos pinned by a user and removes
// the invalid pins. (Needs to be called everytime before we read/write a pin)
func CleanupPins(ctx *context.Context, u *user_model.User) error {
	pinnedRepos, err := repo_model.GetPinnedRepos(*ctx, &repo_model.PinnedReposOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		PinnerID: u.ID,
	})
	if err != nil {
		return err
	}

	for _, repo := range pinnedRepos {
		if !HasPermsToPin(ctx, u, repo) {
			if err := repo_model.PinRepo(*ctx, u, repo, false); err != nil {
				return err
			}
		}
	}

	return nil
}

// Returns the pinned repos of a user that the viewer can see
func GetUserPinnedRepos(ctx *context.Context, user, viewer *user_model.User) ([]*repo_model.Repository, error) {
	// Start by cleaning up the invalid pins
	err := CleanupPins(ctx, user)
	if err != nil {
		return nil, err
	}

	// Get all of the user's pinned repos
	pinnedRepos, err := repo_model.GetPinnedRepos(*ctx, &repo_model.PinnedReposOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		PinnerID: user.ID,
	})
	if err != nil {
		return nil, err
	}

	var repos []*repo_model.Repository

	// Only include the repos that the viewer can see
	for _, repo := range pinnedRepos {
		if canSeePin(ctx, viewer, repo) {
			repos = append(repos, repo)
		}
	}

	return repos, nil
}

func PinRepo(ctx *context.Context, doer *user_model.User, repo *repo_model.Repository, pin, toOrg bool) error {
	// Determine the user which profile is the target for the pin
	var targetUser *user_model.User
	if toOrg {
		targetUser = repo.Owner
	} else {
		targetUser = doer
	}

	// Start by cleaning up the invalid pins
	err := CleanupPins(ctx, targetUser)
	if err != nil {
		return err
	}

	// If target is org profile, need to check if the doer can pin the repo
	// on said org profile
	if toOrg {
		err = assertUserOrgPerms(ctx, doer, repo)
		if err != nil {
			return err
		}
	}

	if pin {
		if !CanPin(ctx, targetUser, repo) {
			return errors.New("user cannot pin this repository")
		}
	}

	return repo_model.PinRepo(*ctx, targetUser, repo, pin)
}

func assertUserOrgPerms(ctx *context.Context, doer *user_model.User, repo *repo_model.Repository) error {
	if !ctx.Repo.Owner.IsOrganization() {
		return errors.New("owner is not an organization")
	}

	isAdmin, err := organization.OrgFromUser(repo.Owner).IsOrgAdmin(ctx, doer.ID)
	if err != nil {
		return err
	}

	if !isAdmin {
		return errors.New("user is not an admin of this organization")
	}

	return nil
}
