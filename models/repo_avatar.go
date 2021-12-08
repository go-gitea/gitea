// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"crypto/md5"
	"fmt"
	"image/png"
	"io"
	"strconv"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// RemoveRandomAvatars removes the randomly generated avatars that were created for repositories
func RemoveRandomAvatars(ctx context.Context) error {
	return db.GetEngine(db.DefaultContext).
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(repo_model.Repository),
			func(idx int, bean interface{}) error {
				repository := bean.(*repo_model.Repository)
				select {
				case <-ctx.Done():
					return db.ErrCancelledf("before random avatars removed for %s", repository.FullName())
				default:
				}
				stringifiedID := strconv.FormatInt(repository.ID, 10)
				if repository.Avatar == stringifiedID {
					return DeleteRepoAvatar(repository)
				}
				return nil
			})
}

// UploadRepoAvatar saves custom avatar for repository.
// FIXME: split uploads to different subdirs in case we have massive number of repos.
func UploadRepoAvatar(repo *repo_model.Repository, data []byte) error {
	m, err := avatar.Prepare(data)
	if err != nil {
		return err
	}

	newAvatar := fmt.Sprintf("%d-%x", repo.ID, md5.Sum(data))
	if repo.Avatar == newAvatar { // upload the same picture
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	oldAvatarPath := repo.CustomAvatarRelativePath()

	// Users can upload the same image to other repo - prefix it with ID
	// Then repo will be removed - only it avatar file will be removed
	repo.Avatar = newAvatar
	if _, err := db.GetEngine(ctx).ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("UploadAvatar: Update repository avatar: %v", err)
	}

	if err := storage.SaveFrom(storage.RepoAvatars, repo.CustomAvatarRelativePath(), func(w io.Writer) error {
		if err := png.Encode(w, *m); err != nil {
			log.Error("Encode: %v", err)
		}
		return err
	}); err != nil {
		return fmt.Errorf("UploadAvatar %s failed: Failed to remove old repo avatar %s: %v", repo.RepoPath(), newAvatar, err)
	}

	if len(oldAvatarPath) > 0 {
		if err := storage.RepoAvatars.Delete(oldAvatarPath); err != nil {
			return fmt.Errorf("UploadAvatar: Failed to remove old repo avatar %s: %v", oldAvatarPath, err)
		}
	}

	return committer.Commit()
}

// DeleteRepoAvatar deletes the repos's custom avatar.
func DeleteRepoAvatar(repo *repo_model.Repository) error {
	// Avatar not exists
	if len(repo.Avatar) == 0 {
		return nil
	}

	avatarPath := repo.CustomAvatarRelativePath()
	log.Trace("DeleteAvatar[%d]: %s", repo.ID, avatarPath)

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	repo.Avatar = ""
	if _, err := db.GetEngine(ctx).ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("DeleteAvatar: Update repository avatar: %v", err)
	}

	if err := storage.RepoAvatars.Delete(avatarPath); err != nil {
		return fmt.Errorf("DeleteAvatar: Failed to remove %s: %v", avatarPath, err)
	}

	return committer.Commit()
}
