// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
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
		ID        int64
		OwnerID   int64
		OwnerName string
		Name      string
	}

	type User struct {
		ID    int64
		Name  string
		Email string
	}

	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	var (
		gitRepoCache = make(map[int64]*git.Repository)
		gitRepo      *git.Repository
		repoCache    = make(map[int64]*Repository)
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

		if err := sess.Begin(); err != nil {
			return err
		}

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
						log.Warn("Release[%d] is orphaned and refers to non-existing repository %d", release.ID, release.RepoID)
						log.Warn("This release should be deleted")
						continue
					}

					if repo.OwnerName == "" {
						// v120.go migration may not have been run correctly - we'll just replicate it here
						// because this appears to be a common-ish problem.
						if _, err := sess.Exec("UPDATE repository SET owner_name = (SELECT name FROM `user` WHERE `user`.id = repository.owner_id)"); err != nil {
							return err
						}

						if _, err := sess.ID(release.RepoID).Get(repo); err != nil {
							return err
						}
					}

					repoCache[release.RepoID] = repo
				}

				if repo.OwnerName == "" {
					log.Warn("Release[%d] refers to Repo[%d] Name: %s with OwnerID: %d but does not have OwnerName set", release.ID, release.RepoID, repo.Name, repo.OwnerID)
					continue
				}

				gitRepo, err = git.OpenRepository(repoPath(repo.OwnerName, repo.Name))
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

		if err := sess.Commit(); err != nil {
			return err
		}
	}

	return nil
}
