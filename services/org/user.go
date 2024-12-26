// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// RemoveOrgUser removes user from given organization.
func RemoveOrgUser(ctx context.Context, org *organization.Organization, user *user_model.User) error {
	ou := new(organization.OrgUser)

	has, err := db.GetEngine(ctx).
		Where("uid=?", user.ID).
		And("org_id=?", org.ID).
		Get(ou)
	if err != nil {
		return fmt.Errorf("get org-user: %w", err)
	} else if !has {
		return nil
	}

	// Check if the user to delete is the last member in owner team.
	if isOwner, err := organization.IsOrganizationOwner(ctx, org.ID, user.ID); err != nil {
		return err
	} else if isOwner {
		t, err := organization.GetOwnerTeam(ctx, org.ID)
		if err != nil {
			return err
		}
		if t.NumMembers == 1 {
			if err := t.LoadMembers(ctx); err != nil {
				return err
			}
			if t.Members[0].ID == user.ID {
				return organization.ErrLastOrgOwner{UID: user.ID}
			}
		}
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err := db.DeleteByID[organization.OrgUser](ctx, ou.ID); err != nil {
		return err
	} else if _, err = db.Exec(ctx, "UPDATE `user` SET num_members=num_members-1 WHERE id=?", org.ID); err != nil {
		return err
	}

	// Delete all repository accesses and unwatch them.
	env, err := repo_model.AccessibleReposEnv(ctx, org, user.ID)
	if err != nil {
		return fmt.Errorf("AccessibleReposEnv: %w", err)
	}
	repoIDs, err := env.RepoIDs(1, org.NumRepos)
	if err != nil {
		return fmt.Errorf("GetUserRepositories [%d]: %w", user.ID, err)
	}
	for _, repoID := range repoIDs {
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return err
		}
		if err = repo_model.WatchRepo(ctx, user, repo, false); err != nil {
			return err
		}
	}

	if len(repoIDs) > 0 {
		if _, err = db.GetEngine(ctx).
			Where("user_id = ?", user.ID).
			In("repo_id", repoIDs).
			Delete(new(access_model.Access)); err != nil {
			return err
		}
	}

	// Delete member in their teams.
	teams, err := organization.GetUserOrgTeams(ctx, org.ID, user.ID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err = removeTeamMember(ctx, t, user); err != nil {
			return err
		}
	}

	return committer.Commit()
}
