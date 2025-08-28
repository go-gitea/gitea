// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"context"
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

func FixPublisherIDforTagReleases(ctx context.Context, x *xorm.Engine) error {
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
		repo    *Repository
		gitRepo *git.Repository
		user    *User
	)
	defer func() {
		if gitRepo != nil {
			gitRepo.Close()
		}
	}()
	for start := 0; ; start += batchSize {
		releases := make([]*Release, 0, batchSize)

		if err := sess.Begin(); err != nil {
			return err
		}

		if err := sess.Limit(batchSize, start).
			Where("publisher_id = 0 OR publisher_id is null").
			Asc("repo_id", "id").Where("is_tag=?", true).
			Find(&releases); err != nil {
			return err
		}

		if len(releases) == 0 {
			break
		}

		for _, release := range releases {
			if repo == nil || repo.ID != release.RepoID {
				if gitRepo != nil {
					gitRepo.Close()
					gitRepo = nil
				}
				repo = new(Repository)
				has, err := sess.ID(release.RepoID).Get(repo)
				if err != nil {
					log.Error("Error whilst loading repository[%d] for release[%d] with tag name %s. Error: %v", release.RepoID, release.ID, release.TagName, err)
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
						log.Error("Error whilst updating repository[%d] owner name", repo.ID)
						return err
					}

					if _, err := sess.ID(release.RepoID).Get(repo); err != nil {
						log.Error("Error whilst loading repository[%d] for release[%d] with tag name %s. Error: %v", release.RepoID, release.ID, release.TagName, err)
						return err
					}
				}
				gitRepo, err = git.OpenRepository(ctx, repoPath(repo.OwnerName, repo.Name))
				if err != nil {
					log.Error("Error whilst opening git repo for [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
					return err
				}
			}

			commit, err := gitRepo.GetTagCommit(release.TagName)
			if err != nil {
				if git.IsErrNotExist(err) {
					log.Warn("Unable to find commit %s for Tag: %s in [%d]%s/%s. Cannot update publisher ID.", err.(git.ErrNotExist).ID, release.TagName, repo.ID, repo.OwnerName, repo.Name)
					continue
				}
				log.Error("Error whilst getting commit for Tag: %s in [%d]%s/%s. Error: %v", release.TagName, repo.ID, repo.OwnerName, repo.Name, err)
				return fmt.Errorf("GetTagCommit: %w", err)
			}

			if commit.Author.Email == "" {
				log.Warn("Tag: %s in Repo[%d]%s/%s does not have a tagger.", release.TagName, repo.ID, repo.OwnerName, repo.Name)
				commit, err = gitRepo.GetCommit(commit.ID.String())
				if err != nil {
					if git.IsErrNotExist(err) {
						log.Warn("Unable to find commit %s for Tag: %s in [%d]%s/%s. Cannot update publisher ID.", err.(git.ErrNotExist).ID, release.TagName, repo.ID, repo.OwnerName, repo.Name)
						continue
					}
					log.Error("Error whilst getting commit for Tag: %s in [%d]%s/%s. Error: %v", release.TagName, repo.ID, repo.OwnerName, repo.Name, err)
					return fmt.Errorf("GetCommit: %w", err)
				}
			}

			if commit.Author.Email == "" {
				log.Warn("Tag: %s in Repo[%d]%s/%s does not have a Tagger and its underlying commit does not have an Author either!", release.TagName, repo.ID, repo.OwnerName, repo.Name)
				continue
			}

			if user == nil || !strings.EqualFold(user.Email, commit.Author.Email) {
				user = new(User)
				_, err = sess.Where("email=?", commit.Author.Email).Get(user)
				if err != nil {
					log.Error("Error whilst getting commit author by email: %s for Tag: %s in [%d]%s/%s. Error: %v", commit.Author.Email, release.TagName, repo.ID, repo.OwnerName, repo.Name, err)
					return err
				}

				user.Email = commit.Author.Email
			}

			if user.ID <= 0 {
				continue
			}

			release.PublisherID = user.ID
			if _, err := sess.ID(release.ID).Cols("publisher_id").Update(release); err != nil {
				log.Error("Error whilst updating publisher[%d] for release[%d] with tag name %s. Error: %v", release.PublisherID, release.ID, release.TagName, err)
				return err
			}
		}
		if gitRepo != nil {
			gitRepo.Close()
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}

	return nil
}
