// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
)

// RemoveOrgUser removes user from given organization.
func RemoveOrgUser(ctx context.Context, orgID, userID int64) error {
	ou := new(organization.OrgUser)

	has, err := db.GetEngine(ctx).
		Where("uid=?", userID).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return fmt.Errorf("get org-user: %w", err)
	} else if !has {
		return nil
	}

	org, err := organization.GetOrgByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %w", orgID, err)
	}

	// Check if the user to delete is the last member in owner team.
	if isOwner, err := organization.IsOrganizationOwner(ctx, orgID, userID); err != nil {
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
			if t.Members[0].ID == userID {
				return organization.ErrLastOrgOwner{UID: userID}
			}
		}
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err := db.GetEngine(ctx).ID(ou.ID).Delete(ou); err != nil {
		return err
	} else if _, err = db.Exec(ctx, "UPDATE `user` SET num_members=num_members-1 WHERE id=?", orgID); err != nil {
		return err
	}

	// Delete all repository accesses and unwatch them.
	env, err := organization.AccessibleReposEnv(ctx, org, userID)
	if err != nil {
		return fmt.Errorf("AccessibleReposEnv: %w", err)
	}
	repoIDs, err := env.RepoIDs(1, org.NumRepos)
	if err != nil {
		return fmt.Errorf("GetUserRepositories [%d]: %w", userID, err)
	}
	for _, repoID := range repoIDs {
		if err = repo_model.WatchRepo(ctx, userID, repoID, false); err != nil {
			return err
		}
	}

	if len(repoIDs) > 0 {
		if _, err = db.GetEngine(ctx).
			Where("user_id = ?", userID).
			In("repo_id", repoIDs).
			Delete(new(access_model.Access)); err != nil {
			return err
		}
	}

	// Delete member in their teams.
	teams, err := organization.GetUserOrgTeams(ctx, org.ID, userID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err = removeTeamMember(ctx, t, userID); err != nil {
			return err
		}
	}

	return committer.Commit()
}
