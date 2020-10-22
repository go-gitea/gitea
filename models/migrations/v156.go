// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

// Copy paste from models/repo.go because we cannot import models package
func repoPath(userName, repoName string) string {
	return filepath.Join(userPath(userName), strings.ToLower(repoName)+".git")
}

func userPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

func fixPublisherIDforTagReleases(x *xorm.Engine) error {

	type Release struct {
		ID          int64
		RepoID      int64
		Sha1        string
		TagName     string
		PublisherID int64
	}

	type Repository struct {
		ID      int64
		OwnerID int64
		Name    string
	}

	type User struct {
		ID    int64
		Name  string
		Email string
	}

	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var (
		gitRepoCache = make(map[int64]*git.Repository)
		gitRepo      *git.Repository
		repoCache    = make(map[int64]*Repository)
		userCache    = make(map[int64]*User)
		ok           bool
		err          error
	)
	defer func() {
		for i := range gitRepoCache {
			gitRepoCache[i].Close()
		}
	}()
	for start := 0; ; start += batchSize {
		releases := make([]*Release, 0, batchSize)

		if err := sess.Limit(batchSize, start).Asc("id").Where("is_tag=?", true).Find(&releases); err != nil {
			return err
		}

		if len(releases) == 0 {
			break
		}

		for _, release := range releases {
			gitRepo, ok = gitRepoCache[release.RepoID]
			if !ok {
				repo, ok := repoCache[release.RepoID]
				if !ok {
					repo = new(Repository)
					has, err := sess.ID(release.RepoID).Get(repo)
					if err != nil {
						return err
					} else if !has {
						return fmt.Errorf("Repository %d is not exist", release.RepoID)
					}

					repoCache[release.RepoID] = repo
				}

				user, ok := userCache[repo.OwnerID]
				if !ok {
					user = new(User)
					has, err := sess.ID(repo.OwnerID).Get(user)
					if err != nil {
						return err
					} else if !has {
						return fmt.Errorf("User %d is not exist", repo.OwnerID)
					}

					userCache[repo.OwnerID] = user
				}

				gitRepo, err = git.OpenRepository(repoPath(user.Name, repo.Name))
				if err != nil {
					return err
				}
				gitRepoCache[release.RepoID] = gitRepo
			}

			commit, err := gitRepo.GetTagCommit(release.TagName)
			if err != nil {
				return fmt.Errorf("GetTagCommit: %v", err)
			}

			u := new(User)
			exists, err := sess.Where("email=?", commit.Author.Email).Get(u)
			if err != nil {
				return err
			}

			if !exists {
				continue
			}

			release.PublisherID = u.ID
			if _, err := sess.ID(release.ID).Cols("publisher_id").Update(release); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}
