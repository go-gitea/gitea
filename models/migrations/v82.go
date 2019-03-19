// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"github.com/go-xorm/xorm"
)

func fixReleaseSha1OnReleaseTable(x *xorm.Engine) error {
	type Release struct {
		ID      int64
		RepoID  int64
		Sha1    string
		TagName string
	}

	// Update release sha1
	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	var (
		err          error
		count        int
		gitRepoCache = make(map[int64]*git.Repository)
		repoCache    = make(map[int64]*models.Repository)
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
					repo, err = models.GetRepositoryByID(release.RepoID)
					if err != nil {
						return err
					}
					repoCache[release.RepoID] = repo
				}

				gitRepo, err = git.OpenRepository(repo.RepoPath())
				if err != nil {
					return err
				}
				gitRepoCache[release.RepoID] = gitRepo
			}

			release.Sha1, err = gitRepo.GetTagCommitID(release.TagName)
			if err != nil {
				return err
			}

			if _, err = sess.ID(release.ID).Cols("sha1").Update(release); err != nil {
				return err
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
