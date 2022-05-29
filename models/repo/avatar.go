// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// CustomAvatarRelativePath returns repository custom avatar file path.
func (repo *Repository) CustomAvatarRelativePath() string {
	return repo.Avatar
}

// RelAvatarLink returns a relative link to the repository's avatar.
func (repo *Repository) RelAvatarLink() string {
	return repo.relAvatarLink(db.DefaultContext)
}

// generateRandomAvatar generates a random avatar for repository.
func generateRandomAvatar(ctx context.Context, repo *Repository) error {
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

	if _, err := db.GetEngine(ctx).ID(repo.ID).Cols("avatar").NoAutoTime().Update(repo); err != nil {
		return err
	}

	return nil
}

func (repo *Repository) relAvatarLink(ctx context.Context) string {
	// If no avatar - path is empty
	avatarPath := repo.CustomAvatarRelativePath()
	if len(avatarPath) == 0 {
		switch mode := setting.RepoAvatar.Fallback; mode {
		case "image":
			return setting.RepoAvatar.FallbackImage
		case "random":
			if err := generateRandomAvatar(ctx, repo); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	return setting.AppSubURL + "/repo-avatars/" + url.PathEscape(repo.Avatar)
}

// AvatarLink returns a link to the repository's avatar.
func (repo *Repository) AvatarLink() string {
	return repo.avatarLink(db.DefaultContext)
}

// avatarLink returns user avatar absolute link.
func (repo *Repository) avatarLink(ctx context.Context) string {
	link := repo.relAvatarLink(ctx)
	// we only prepend our AppURL to our known (relative, internal) avatar link to get an absolute URL
	if strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") {
		return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
	}
	// otherwise, return the link as it is
	return link
}
