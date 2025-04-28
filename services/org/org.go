// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/storage"
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
