// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
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
			// Prepare Notify
			if err := rel.LoadAttributes(); err != nil {
				log.Error("LoadAttributes: %v", err)
				return err
			}

			refName := git.TagPrefix + rel.TagName
			apiPusher := rel.Publisher.APIFormat()
			apiRepo := rel.Repo.APIFormat(models.AccessModeNone)
			apiCommits, err := models.NewPushCommits().ToAPIPayloadCommits(rel.Repo.RepoPath(), rel.Repo.HTMLURL())
			if err != nil {
				log.Error("commits.ToAPIPayloadCommits failed: %v", err)
				return err
			}

			if err := models.PrepareWebhooks(rel.Repo, models.HookEventPush, &api.PushPayload{
				Ref:        refName,
				Before:     git.EmptySHA,
				After:      commit.ID.String(),
				CompareURL: setting.AppURL + models.NewPushCommits().CompareURL,
				Commits:    apiCommits,
				Repo:       apiRepo,
				Pusher:     apiPusher,
				Sender:     apiPusher,
			}); err != nil {
				log.Error("PrepareWebhooks: %v", err)
			}

			gitRepo, err := git.OpenRepository(rel.Repo.RepoPath())
			if err != nil {
				log.Error("OpenRepository[%s]: %v", rel.Repo.RepoPath(), err)
			}
			shaSum, err := gitRepo.GetTagCommitID(refName)
			if err != nil {
				gitRepo.Close()
				log.Error("GetTagCommitID[%s]: %v", refName, err)
			}
			gitRepo.Close()

			if err = models.PrepareWebhooks(rel.Repo, models.HookEventCreate, &api.CreatePayload{
				Ref:     git.RefEndName(refName),
				Sha:     shaSum,
				RefType: "tag",
				Repo:    apiRepo,
				Sender:  apiPusher,
			}); err != nil {
				return fmt.Errorf("PrepareWebhooks: %v", err)
			}
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

	if err = models.AddReleaseAttachments(rel.ID, attachmentUUIDs); err != nil {
		log.Error("AddReleaseAttachments: %v", err)
	}

	if err = rel.LoadAttributes(); err != nil {
		return err
	}

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

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(id int64, doer *models.User, delTag bool) error {
	rel, err := models.GetReleaseByID(id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %v", err)
	}

	repo, err := models.GetRepositoryByID(rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}

	if delTag {
		_, stderr, err := process.GetManager().ExecDir(-1, repo.RepoPath(),
			fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID),
			git.GitExecutable, "tag", "-d", rel.TagName)
		if err != nil && !strings.Contains(stderr, "not found") {
			return fmt.Errorf("git tag -d: %v - %s", err, stderr)
		}

		if err := models.DeleteReleaseByID(id); err != nil {
			return fmt.Errorf("DeleteReleaseByID: %v", err)
		}
	} else {
		rel.IsTag = true
		rel.IsDraft = false
		rel.IsPrerelease = false
		rel.Title = ""
		rel.Note = ""

		if err = models.UpdateRelease(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}

	rel.Repo = repo
	if err = rel.LoadAttributes(); err != nil {
		return fmt.Errorf("LoadAttributes: %v", err)
	}

	if err := models.DeleteAttachmentsByRelease(rel.ID); err != nil {
		return fmt.Errorf("DeleteAttachments: %v", err)
	}

	for i := range rel.Attachments {
		attachment := rel.Attachments[i]
		if err := os.RemoveAll(attachment.LocalPath()); err != nil {
			log.Error("Delete attachment %s of release %s failed: %v", attachment.UUID, rel.ID, err)
		}
	}

	mode, _ := models.AccessLevel(doer, rel.Repo)
	if err := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseDeleted,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(rel.Repo.ID)
	}

	return nil
}
