// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
)

// deleteOrganization deletes models associated to an organization.
func deleteOrganization(ctx context.Context, org *org_model.Organization) error {
	if org.Type != user_model.UserTypeOrganization {
		return fmt.Errorf("%s is a user not an organization", org.Name)
	}

	if err := db.DeleteBeans(ctx,
		&org_model.Team{OrgID: org.ID},
		&org_model.OrgUser{OrgID: org.ID},
		&org_model.TeamUser{OrgID: org.ID},
		&org_model.TeamUnit{OrgID: org.ID},
		&org_model.TeamInvite{OrgID: org.ID},
		&secret_model.Secret{OwnerID: org.ID},
		&user_model.Blocking{BlockerID: org.ID},
		&actions_model.ActionRunner{OwnerID: org.ID},
		&actions_model.ActionRunnerToken{OwnerID: org.ID},
	); err != nil {
		return fmt.Errorf("DeleteBeans: %w", err)
	}

	if _, err := db.GetEngine(ctx).ID(org.ID).Delete(new(user_model.User)); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}

	return nil
}

// DeleteOrganization completely and permanently deletes everything of organization.
func DeleteOrganization(ctx context.Context, org *org_model.Organization, purge bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if purge {
		err := repo_service.DeleteOwnerRepositoriesDirectly(ctx, org.AsUser())
		if err != nil {
			return err
		}
	}

	// Check ownership of repository.
	count, err := repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{OwnerID: org.ID})
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %w", err)
	} else if count > 0 {
		return repo_model.ErrUserOwnRepos{UID: org.ID}
	}

	// Check ownership of packages.
	if ownsPackages, err := packages_model.HasOwnerPackages(ctx, org.ID); err != nil {
		return fmt.Errorf("HasOwnerPackages: %w", err)
	} else if ownsPackages {
		return packages_model.ErrUserOwnPackages{UID: org.ID}
	}

	if err := deleteOrganization(ctx, org); err != nil {
		return fmt.Errorf("DeleteOrganization: %w", err)
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	// FIXME: system notice
	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := user_model.UserPath(org.Name)

	if err := util.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to RemoveAll %s: %w", path, err)
	}

	if len(org.Avatar) > 0 {
		avatarPath := org.CustomAvatarRelativePath()
		if err := storage.Avatars.Delete(avatarPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", avatarPath, err)
		}
	}

	return nil
}

func updateOrgRepoForVisibilityChanged(ctx context.Context, repo *repo_model.Repository, makePrivate bool) error {
	// Organization repository need to recalculate access table when visibility is changed.
	if err := access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
		return fmt.Errorf("recalculateTeamAccesses: %w", err)
	}

	if makePrivate {
		if _, err := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).Cols("is_private").Update(&activities_model.Action{
			IsPrivate: true,
		}); err != nil {
			return err
		}

		if err := repo_model.ClearRepoStars(ctx, repo.ID); err != nil {
			return err
		}
	}

	// Create/Remove git-daemon-export-ok for git-daemon...
	if err := repo_service.CheckDaemonExportOK(ctx, repo); err != nil {
		return err
	}

	// If visibility is changed, we need to update the issue indexer.
	// Since the data in the issue indexer have field to indicate if the repo is public or not.
	// FIXME: it should check organization visibility instead of repository visibility only.
	issue_indexer.UpdateRepoIndexer(ctx, repo.ID)

	forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("getRepositoriesByForkID: %w", err)
	}
	for i := range forkRepos {
		if err := updateOrgRepoForVisibilityChanged(ctx, forkRepos[i], makePrivate); err != nil {
			return fmt.Errorf("updateRepoForVisibilityChanged[%s]: %w", forkRepos[i].FullName(), err)
		}
	}
	return nil
}

func ChangeOrganizationVisibility(ctx context.Context, org *org_model.Organization, visibility structs.VisibleType) error {
	if org.Visibility == visibility {
		return nil
	}

	org.Visibility = visibility
	// FIXME: If it's a big forks network(forks and sub forks), the database transaction will be too long to fail.
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := user_model.UpdateUserColsNoAutoTime(ctx, org.AsUser(), "visibility"); err != nil {
			return err
		}

		repos, _, err := repo_model.GetUserRepositories(ctx, repo_model.SearchRepoOptions{
			Actor: org.AsUser(), Private: true, ListOptions: db.ListOptionsAll,
		})
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if err := updateOrgRepoForVisibilityChanged(ctx, repo, visibility == structs.VisibleTypePrivate); err != nil {
				return fmt.Errorf("updateOrgRepoForVisibilityChanged: %w", err)
			}
		}
		return nil
	})
}
