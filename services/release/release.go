// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

func createTag(gitRepo *git.Repository, rel *models.Release) error {
	// Only actual create when publish.
	if !rel.IsDraft {
		if !gitRepo.IsTagExist(rel.TagName) {
			commit, err := gitRepo.GetCommit(rel.Target)
			if err != nil {
				return fmt.Errorf("GetCommit: %v", err)
			}

			// Trim '--' prefix to prevent command line argument vulnerability.
			rel.TagName = strings.TrimPrefix(rel.TagName, "--")
			if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return models.ErrInvalidTagName{
						TagName: rel.TagName,
					}
				}
				return err
			}
			rel.LowerTagName = strings.ToLower(rel.TagName)
		}
		commit, err := gitRepo.GetTagCommit(rel.TagName)
		if err != nil {
			return fmt.Errorf("GetTagCommit: %v", err)
		}

		rel.Sha1 = commit.ID.String()
		rel.CreatedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		rel.NumCommits, err = commit.CommitsCount()
		if err != nil {
			return fmt.Errorf("CommitsCount: %v", err)
		}
	} else {
		rel.CreatedUnix = timeutil.TimeStampNow()
	}
	return nil
}

// CreateRelease creates a new release of repository.
func CreateRelease(gitRepo *git.Repository, rel *models.Release, attachmentUUIDs []string) error {
	isExist, err := models.IsReleaseExist(rel.RepoID, rel.TagName)
	if err != nil {
		return err
	} else if isExist {
		return models.ErrReleaseAlreadyExist{
			TagName: rel.TagName,
		}
	}

	if err = createTag(gitRepo, rel); err != nil {
		return err
	}

	rel.LowerTagName = strings.ToLower(rel.TagName)
	if err = models.InsertRelease(rel); err != nil {
		return err
	}

	if err = models.AddReleaseAttachments(rel.ID, attachmentUUIDs); err != nil {
		return err
	}

	if !rel.IsDraft {
		if err := rel.LoadAttributes(); err != nil {
			log.Error("LoadAttributes: %v", err)
		} else {
			mode, _ := models.AccessLevel(rel.Publisher, rel.Repo)
			if err := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
				Action:     api.HookReleasePublished,
				Release:    rel.APIFormat(),
				Repository: rel.Repo.APIFormat(mode),
				Sender:     rel.Publisher.APIFormat(),
			}); err != nil {
				log.Error("PrepareWebhooks: %v", err)
			} else {
				go models.HookQueue.Add(rel.Repo.ID)
			}
		}
	}

	return nil
}

// UpdateRelease updates information of a release.
func UpdateRelease(doer *models.User, gitRepo *git.Repository, rel *models.Release, attachmentUUIDs []string) (err error) {
	if err = createTag(gitRepo, rel); err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)

	if err = models.UpdateRelease(rel); err != nil {
		return err
	}

	if err = rel.LoadAttributes(); err != nil {
		return err
	}

	err = models.AddReleaseAttachments(rel.ID, attachmentUUIDs)

	// even if attachments added failed, hooks will be still triggered
	mode, _ := models.AccessLevel(doer, rel.Repo)
	if err1 := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseUpdated,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err1 != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(rel.Repo.ID)
	}

	return err
}
