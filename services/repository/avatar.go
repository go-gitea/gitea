// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// UploadAvatar saves custom avatar for repository.
// FIXME: split uploads to different subdirs in case we have massive number of repos.
func UploadAvatar(ctx context.Context, repo *repo_model.Repository, data []byte) (string, error) {
	avatarData, err := avatar.ProcessAvatarImage(data)
	oldAvatarHash := ""
	if err != nil {
		return oldAvatarHash, fmt.Errorf("UploadAvatar: failed to process repo avatar image: %w", err)
	}

	result := db.WithTx(ctx, func(ctx context.Context) error {
		if err := repo_model.RefreshRepositoryCols(ctx, repo, "avatar"); err != nil {
			return fmt.Errorf("UploadAvatar: failed to refresh repository avatar: %w", err)
		}
		oldAvatarPath := repo.CustomAvatarRelativePath()
		// Users can upload the same image to other repo - prefix it with ID
		// Then repo will be removed - only it avatar file will be removed
		newAvatar := avatar.HashAvatar(repo.ID, data)
		oldAvatarHash = repo.Avatar
		if oldAvatarHash == newAvatar { // upload the same picture
			return nil
		}

		repo.Avatar = newAvatar
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "avatar"); err != nil {
			return fmt.Errorf("UploadAvatar: failed to update repository avatar: %w", err)
		}

		if err := storage.SaveFrom(storage.RepoAvatars, repo.CustomAvatarRelativePath(), func(w io.Writer) error {
			_, err := w.Write(avatarData)
			return err
		}); err != nil {
			return fmt.Errorf("UploadAvatar: failed to save repo avatar %s: %w", newAvatar, err)
		}

		if len(oldAvatarPath) > 0 {
			if err := storage.RepoAvatars.Delete(oldAvatarPath); err != nil {
				log.Warn("UploadAvatar: failed to remove old repo avatar %s: %w", oldAvatarPath, err)
			}
		}
		return nil
	})

	return oldAvatarHash, result
}

// DeleteAvatar deletes the repos's custom avatar.
func DeleteAvatar(ctx context.Context, repo *repo_model.Repository) (string, error) {
	oldAvatarHash := ""
	result := db.WithTx(ctx, func(ctx context.Context) error {
		if err := repo_model.RefreshRepositoryCols(ctx, repo, "avatar"); err != nil {
			return fmt.Errorf("DeleteAvatar: Refresh repository avatar: %w", err)
		}

		oldAvatarPath := repo.CustomAvatarRelativePath()
		log.Trace("DeleteAvatar[%d]: %s", repo.ID, oldAvatarPath)
		oldAvatarHash = repo.Avatar
		repo.Avatar = ""
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "avatar"); err != nil {
			return fmt.Errorf("DeleteAvatar: Update repository avatar: %w", err)
		}

		if len(oldAvatarPath) > 0 {
			if err := storage.RepoAvatars.Delete(oldAvatarPath); err != nil {
				return fmt.Errorf("DeleteAvatar: Failed to remove %s: %w", oldAvatarPath, err)
			}
		}
		return nil
	})

	return oldAvatarHash, result
}

// RemoveRandomAvatars removes the randomly generated avatars that were created for repositories
func RemoveRandomAvatars(ctx context.Context) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, repository *repo_model.Repository) error {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("before random avatars removed for %s", repository.FullName())
		default:
		}
		stringifiedID := strconv.FormatInt(repository.ID, 10)
		if repository.Avatar == stringifiedID {
			_, err := DeleteAvatar(ctx, repository)
			return err
		}
		return nil
	})
}

// generateAvatar generates the avatar from a template repository
func generateAvatar(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	// generate a new different hash, whatever the "hash data" is, it doesn't matter
	generateRepo.Avatar = avatar.HashAvatar(generateRepo.ID, []byte("new-avatar"))
	if _, err := storage.Copy(storage.RepoAvatars, generateRepo.CustomAvatarRelativePath(), storage.RepoAvatars, templateRepo.CustomAvatarRelativePath()); err != nil {
		return err
	}

	return repo_model.UpdateRepositoryColsNoAutoTime(ctx, generateRepo, "avatar")
}
