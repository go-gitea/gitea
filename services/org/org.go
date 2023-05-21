// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/agit"
	container_service "code.gitea.io/gitea/services/packages/container"
)

// DeleteOrganization completely and permanently deletes everything of organization.
func DeleteOrganization(org *org_model.Organization) error {
	ctx, commiter, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer commiter.Close()

	// Check ownership of repository.
	count, err := repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{OwnerID: org.ID})
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %w", err)
	} else if count > 0 {
		return models.ErrUserOwnRepos{UID: org.ID}
	}

	// Check ownership of packages.
	if ownsPackages, err := packages_model.HasOwnerPackages(ctx, org.ID); err != nil {
		return fmt.Errorf("HasOwnerPackages: %w", err)
	} else if ownsPackages {
		return models.ErrUserOwnPackages{UID: org.ID}
	}

	if err := org_model.DeleteOrganization(ctx, org); err != nil {
		return fmt.Errorf("DeleteOrganization: %w", err)
	}

	if err := commiter.Commit(); err != nil {
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

// RenameOrganization renames an organization.
func RenameOrganization(ctx context.Context, org *org_model.Organization, newName string) error {
	if !org.AsUser().IsOrganization() {
		return fmt.Errorf("cannot rename user")
	}

	if err := user_model.IsUsableUsername(newName); err != nil {
		return err
	}

	onlyCapitalization := strings.EqualFold(org.Name, newName)
	oldName := org.Name

	if onlyCapitalization {
		org.Name = newName
		if err := user_model.UpdateUserCols(ctx, org.AsUser(), "name"); err != nil {
			org.Name = oldName
			return err
		}
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	isExist, err := user_model.IsUserExist(ctx, org.ID, newName)
	if err != nil {
		return err
	}
	if isExist {
		return user_model.ErrUserAlreadyExist{
			Name: newName,
		}
	}

	if err = repo_model.UpdateRepositoryOwnerName(ctx, oldName, newName); err != nil {
		return err
	}

	if err = user_model.NewUserRedirect(ctx, org.ID, oldName, newName); err != nil {
		return err
	}

	if err := agit.UserNameChanged(ctx, org.AsUser(), newName); err != nil {
		return err
	}
	if err := container_service.UpdateRepositoryNames(ctx, org.AsUser(), newName); err != nil {
		return err
	}

	org.Name = newName
	org.LowerName = strings.ToLower(newName)
	if err := user_model.UpdateUserCols(ctx, org.AsUser(), "name", "lower_name"); err != nil {
		org.Name = oldName
		org.LowerName = strings.ToLower(oldName)
		return err
	}

	// Do not fail if directory does not exist
	if err = util.Rename(user_model.UserPath(oldName), user_model.UserPath(newName)); err != nil && !os.IsNotExist(err) {
		org.Name = oldName
		org.LowerName = strings.ToLower(oldName)
		return fmt.Errorf("rename user directory: %w", err)
	}

	if err = committer.Commit(); err != nil {
		org.Name = oldName
		org.LowerName = strings.ToLower(oldName)
		if err2 := util.Rename(user_model.UserPath(newName), user_model.UserPath(oldName)); err2 != nil && !os.IsNotExist(err2) {
			log.Critical("Unable to rollback directory change during failed username change from: %s to: %s. DB Error: %v. Filesystem Error: %v", oldName, newName, err, err2)
			return fmt.Errorf("failed to rollback directory change during failed username change from: %s to: %s. DB Error: %w. Filesystem Error: %v", oldName, newName, err, err2)
		}
		return err
	}

	log.Trace("Org name changed: %s -> %s", oldName, newName)
	return nil
}
