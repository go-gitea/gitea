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
	"strings"

	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// CustomAvatarRelativePath returns repository custom avatar file path.
func (repo *Repository) CustomAvatarRelativePath() string {
	return repo.Avatar
}

// generateRandomAvatar generates a random avatar for repository.
func (repo *Repository) generateRandomAvatar(e Engine) error {
	idToString := fmt.Sprintf("%d", repo.ID)

	seed := idToString
	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %v", err)
	}

	repo.Avatar = idToString

	if err := storage.SaveFrom(storage.RepoAvatars, repo.CustomAvatarRelativePath(), func(w io.Writer) error {
		if err := png.Encode(w, img); err != nil {
			log.Error("Encode: %v", err)
		}
		return err
	}); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", repo.CustomAvatarRelativePath(), err)
	}

	log.Info("New random avatar created for repository: %d", repo.ID)

	if _, err := e.ID(repo.ID).Cols("avatar").NoAutoTime().Update(repo); err != nil {
		return err
	}

	return nil
}

// RemoveRandomAvatars removes the randomly generated avatars that were created for repositories
func RemoveRandomAvatars(ctx context.Context) error {
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repository := bean.(*Repository)
				select {
				case <-ctx.Done():
					return ErrCancelledf("before random avatars removed for %s", repository.FullName())
				default:
				}
				stringifiedID := strconv.FormatInt(repository.ID, 10)
				if repository.Avatar == stringifiedID {
					return repository.DeleteAvatar()
				}
				return nil
			})
}

// RelAvatarLink returns a relative link to the repository's avatar.
func (repo *Repository) RelAvatarLink() string {
	return repo.relAvatarLink(x)
}

func (repo *Repository) relAvatarLink(e Engine) string {
	// If no avatar - path is empty
	avatarPath := repo.CustomAvatarRelativePath()
	if len(avatarPath) == 0 {
		switch mode := setting.RepoAvatar.Fallback; mode {
		case "image":
			return setting.RepoAvatar.FallbackImage
		case "random":
			if err := repo.generateRandomAvatar(e); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	return setting.AppSubURL + "/repo-avatars/" + repo.Avatar
}

// AvatarLink returns a link to the repository's avatar.
func (repo *Repository) AvatarLink() string {
	return repo.avatarLink(x)
}

// avatarLink returns user avatar absolute link.
func (repo *Repository) avatarLink(e Engine) string {
	link := repo.relAvatarLink(e)
	// link may be empty!
	if len(link) > 0 {
		if link[0] == '/' && link[1] != '/' {
			return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
		}
	}
	return link
}

// UploadAvatar saves custom avatar for repository.
// FIXME: split uploads to different subdirs in case we have massive number of repos.
func (repo *Repository) UploadAvatar(data []byte) error {
	m, err := avatar.Prepare(data)
	if err != nil {
		return err
	}

	newAvatar := fmt.Sprintf("%d-%x", repo.ID, md5.Sum(data))
	if repo.Avatar == newAvatar { // upload the same picture
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	oldAvatarPath := repo.CustomAvatarRelativePath()

	// Users can upload the same image to other repo - prefix it with ID
	// Then repo will be removed - only it avatar file will be removed
	repo.Avatar = newAvatar
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
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

	return sess.Commit()
}

// DeleteAvatar deletes the repos's custom avatar.
func (repo *Repository) DeleteAvatar() error {
	// Avatar not exists
	if len(repo.Avatar) == 0 {
		return nil
	}

	avatarPath := repo.CustomAvatarRelativePath()
	log.Trace("DeleteAvatar[%d]: %s", repo.ID, avatarPath)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	repo.Avatar = ""
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("DeleteAvatar: Update repository avatar: %v", err)
	}

	if err := storage.RepoAvatars.Delete(avatarPath); err != nil {
		return fmt.Errorf("DeleteAvatar: Failed to remove %s: %v", avatarPath, err)
	}

	return sess.Commit()
}
