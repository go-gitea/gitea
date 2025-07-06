// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func FixReleaseSha1OnReleaseTable(x *xorm.Engine) error {
	type Release struct {
		ID      int64
		RepoID  int64
		Sha1    string
		TagName string
	}

	type Repository struct {
		ID      int64
		OwnerID int64
		Name    string
	}

	type User struct {
		ID   int64
		Name string
	}

	// UserPath returns the path absolute path of user repositories.
	UserPath := func(userName string) string {
		return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
	}

	// RepoPath returns repository path by given user and repository name.
	RepoPath := func(userName, repoName string) string {
		return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".git")
	}

	// Update release sha1
	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	var (
		err          error
		count        int
		gitRepoCache = make(map[int64]*git.Repository)
		repoCache    = make(map[int64]*Repository)
		userCache    = make(map[int64]*User)
	)

	if err = sess.Begin(); err != nil {
		return err
	}

	for start := 0; ; start += batchSize {
		releases := make([]*Release, 0, batchSize)
		if err = sess.Limit(batchSize, start).Asc("id").Where("is_tag=?", false).Find(&releases); err != nil {
			return err
		}
		if len(releases) == 0 {
			break
		}

		for _, release := range releases {
			gitRepo, ok := gitRepoCache[release.RepoID]
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

				gitRepo, err = git.OpenRepository(git.DefaultContext, RepoPath(user.Name, repo.Name))
				if err != nil {
					return err
				}
				defer gitRepo.Close()
				gitRepoCache[release.RepoID] = gitRepo
			}

			release.Sha1, err = gitRepo.GetTagCommitID(release.TagName)
			if err != nil && !git.IsErrNotExist(err) {
				return err
			}

			if err == nil {
				if _, err = sess.ID(release.ID).Cols("sha1").Update(release); err != nil {
					return err
				}
			}

			count++
			if count >= 1000 {
				if err = sess.Commit(); err != nil {
					return err
				}
				if err = sess.Begin(); err != nil {
					return err
				}
				count = 0
			}
		}
	}
	return sess.Commit()
}
