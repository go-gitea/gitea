// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

func createTag(ctx context.Context, gitRepo *git.Repository, rel *repo_model.Release, msg string) (bool, error) {
	var created bool
	// Only actual create when publish.
	if !rel.IsDraft {
		if !gitRepo.IsTagExist(rel.TagName) {
			if err := rel.LoadAttributes(ctx); err != nil {
				log.Error("LoadAttributes: %v", err)
				return false, err
			}

			protectedTags, err := git_model.GetProtectedTags(ctx, rel.Repo.ID)
			if err != nil {
				return false, fmt.Errorf("GetProtectedTags: %w", err)
			}

			// Trim '--' prefix to prevent command line argument vulnerability.
			rel.TagName = strings.TrimPrefix(rel.TagName, "--")
			isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, protectedTags, rel.TagName, rel.PublisherID)
			if err != nil {
				return false, err
			}
			if !isAllowed {
				return false, models.ErrProtectedTagName{
					TagName: rel.TagName,
				}
			}

			commit, err := gitRepo.GetCommit(rel.Target)
			if err != nil {
				return false, fmt.Errorf("createTag::GetCommit[%v]: %w", rel.Target, err)
			}

			if len(msg) > 0 {
				if err = gitRepo.CreateAnnotatedTag(rel.TagName, msg, commit.ID.String()); err != nil {
					if strings.Contains(err.Error(), "is not a valid tag name") {
						return false, models.ErrInvalidTagName{
							TagName: rel.TagName,
						}
					}
					return false, err
				}
			} else if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return false, models.ErrInvalidTagName{
						TagName: rel.TagName,
					}
				}
				return false, err
			}
			created = true
			rel.LowerTagName = strings.ToLower(rel.TagName)

			commits := repository.NewPushCommits()
			commits.HeadCommit = repository.CommitToPushCommit(commit)
			commits.CompareURL = rel.Repo.ComposeCompareURL(git.EmptySHA, commit.ID.String())

			notification.NotifyPushCommits(
				ctx, rel.Publisher, rel.Repo,
				&repository.PushUpdateOptions{
					RefFullName: git.TagPrefix + rel.TagName,
					OldCommitID: git.EmptySHA,
					NewCommitID: commit.ID.String(),
				}, commits)
			notification.NotifyCreateRef(ctx, rel.Publisher, rel.Repo, "tag", git.TagPrefix+rel.TagName, commit.ID.String())
			rel.CreatedUnix = timeutil.TimeStampNow()
		}
		commit, err := gitRepo.GetTagCommit(rel.TagName)
		if err != nil {
			return false, fmt.Errorf("GetTagCommit: %w", err)
		}

		rel.Sha1 = commit.ID.String()
		rel.NumCommits, err = commit.CommitsCount()
		if err != nil {
			return false, fmt.Errorf("CommitsCount: %w", err)
		}

		if rel.PublisherID <= 0 {
			u, err := user_model.GetUserByEmail(ctx, commit.Author.Email)
			if err == nil {
				rel.PublisherID = u.ID
			}
		}
	} else {
		rel.CreatedUnix = timeutil.TimeStampNow()
	}
	return created, nil
}

// CreateRelease creates a new release of repository.
func CreateRelease(gitRepo *git.Repository, rel *repo_model.Release, attachmentUUIDs []string, msg string) error {
	has, err := repo_model.IsReleaseExist(gitRepo.Ctx, rel.RepoID, rel.TagName)
	if err != nil {
		return err
	} else if has {
		return repo_model.ErrReleaseAlreadyExist{
			TagName: rel.TagName,
		}
	}

	if _, err = createTag(gitRepo.Ctx, gitRepo, rel, msg); err != nil {
		return err
	}

	rel.LowerTagName = strings.ToLower(rel.TagName)
	if err = db.Insert(gitRepo.Ctx, rel); err != nil {
		return err
	}

	if err = repo_model.AddReleaseAttachments(gitRepo.Ctx, rel.ID, attachmentUUIDs); err != nil {
		return err
	}

	if !rel.IsDraft {
		notification.NotifyNewRelease(gitRepo.Ctx, rel)
	}

	return nil
}

// CreateNewTag creates a new repository tag
func CreateNewTag(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commit, tagName, msg string) error {
	has, err := repo_model.IsReleaseExist(ctx, repo.ID, tagName)
	if err != nil {
		return err
	} else if has {
		return models.ErrTagAlreadyExists{
			TagName: tagName,
		}
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return err
	}
	defer closer.Close()

	rel := &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  doer.ID,
		Publisher:    doer,
		TagName:      tagName,
		Target:       commit,
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}

	if _, err = createTag(ctx, gitRepo, rel, msg); err != nil {
		return err
	}

	return db.Insert(ctx, rel)
}

// UpdateRelease updates information, attachments of a release and will create tag if it's not a draft and tag not exist.
// addAttachmentUUIDs accept a slice of new created attachments' uuids which will be reassigned release_id as the created release
// delAttachmentUUIDs accept a slice of attachments' uuids which will be deleted from the release
// editAttachments accept a map of attachment uuid to new attachment name which will be updated with attachments.
func UpdateRelease(doer *user_model.User, gitRepo *git.Repository, rel *repo_model.Release,
	addAttachmentUUIDs, delAttachmentUUIDs []string, editAttachments map[string]string,
) (err error) {
	if rel.ID == 0 {
		return errors.New("UpdateRelease only accepts an exist release")
	}
	isCreated, err := createTag(gitRepo.Ctx, gitRepo, rel, "")
	if err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = repo_model.UpdateRelease(ctx, rel); err != nil {
		return err
	}

	if err = repo_model.AddReleaseAttachments(ctx, rel.ID, addAttachmentUUIDs); err != nil {
		return fmt.Errorf("AddReleaseAttachments: %w", err)
	}

	deletedUUIDs := make(container.Set[string])
	if len(delAttachmentUUIDs) > 0 {
		// Check attachments
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, delAttachmentUUIDs)
		if err != nil {
			return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %w", delAttachmentUUIDs, err)
		}
		for _, attach := range attachments {
			if attach.ReleaseID != rel.ID {
				return util.SilentWrap{
					Message: "delete attachment of release permission denied",
					Err:     util.ErrPermissionDenied,
				}
			}
			deletedUUIDs.Add(attach.UUID)
		}

		if _, err := repo_model.DeleteAttachments(ctx, attachments, true); err != nil {
			return fmt.Errorf("DeleteAttachments [uuids: %v]: %w", delAttachmentUUIDs, err)
		}
	}

	if len(editAttachments) > 0 {
		updateAttachmentsList := make([]string, 0, len(editAttachments))
		for k := range editAttachments {
			updateAttachmentsList = append(updateAttachmentsList, k)
		}
		// Check attachments
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, updateAttachmentsList)
		if err != nil {
			return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %w", updateAttachmentsList, err)
		}
		for _, attach := range attachments {
			if attach.ReleaseID != rel.ID {
				return util.SilentWrap{
					Message: "update attachment of release permission denied",
					Err:     util.ErrPermissionDenied,
				}
			}
		}

		for uuid, newName := range editAttachments {
			if !deletedUUIDs.Contains(uuid) {
				if err = repo_model.UpdateAttachmentByUUID(ctx, &repo_model.Attachment{
					UUID: uuid,
					Name: newName,
				}, "name"); err != nil {
					return err
				}
			}
		}
	}

	if err = committer.Commit(); err != nil {
		return
	}

	for _, uuid := range delAttachmentUUIDs {
		if err := storage.Attachments.Delete(repo_model.AttachmentRelativePath(uuid)); err != nil {
			// Even delete files failed, but the attachments has been removed from database, so we
			// should not return error but only record the error on logs.
			// users have to delete this attachments manually or we should have a
			// synchronize between database attachment table and attachment storage
			log.Error("delete attachment[uuid: %s] failed: %v", uuid, err)
		}
	}

	if !isCreated {
		notification.NotifyUpdateRelease(gitRepo.Ctx, doer, rel)
		return
	}

	if !rel.IsDraft {
		notification.NotifyNewRelease(gitRepo.Ctx, rel)
	}

	return err
}

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(ctx context.Context, id int64, doer *user_model.User, delTag bool) error {
	rel, err := repo_model.GetReleaseByID(ctx, id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %w", err)
	}

	repo, err := repo_model.GetRepositoryByID(ctx, rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %w", err)
	}

	if delTag {
		protectedTags, err := git_model.GetProtectedTags(ctx, rel.RepoID)
		if err != nil {
			return fmt.Errorf("GetProtectedTags: %w", err)
		}
		isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, protectedTags, rel.TagName, rel.PublisherID)
		if err != nil {
			return err
		}
		if !isAllowed {
			return models.ErrProtectedTagName{
				TagName: rel.TagName,
			}
		}

		if stdout, _, err := git.NewCommand(ctx, "tag", "-d").AddDashesAndList(rel.TagName).
			SetDescription(fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID)).
			RunStdString(&git.RunOpts{Dir: repo.RepoPath()}); err != nil && !strings.Contains(err.Error(), "not found") {
			log.Error("DeleteReleaseByID (git tag -d): %d in %v Failed:\nStdout: %s\nError: %v", rel.ID, repo, stdout, err)
			return fmt.Errorf("git tag -d: %w", err)
		}

		notification.NotifyPushCommits(
			ctx, doer, repo,
			&repository.PushUpdateOptions{
				RefFullName: git.TagPrefix + rel.TagName,
				OldCommitID: rel.Sha1,
				NewCommitID: git.EmptySHA,
			}, repository.NewPushCommits())
		notification.NotifyDeleteRef(ctx, doer, repo, "tag", git.TagPrefix+rel.TagName)

		if err := repo_model.DeleteReleaseByID(ctx, id); err != nil {
			return fmt.Errorf("DeleteReleaseByID: %w", err)
		}
	} else {
		rel.IsTag = true

		if err = repo_model.UpdateRelease(ctx, rel); err != nil {
			return fmt.Errorf("Update: %w", err)
		}
	}

	rel.Repo = repo
	if err = rel.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: %w", err)
	}

	if err := repo_model.DeleteAttachmentsByRelease(ctx, rel.ID); err != nil {
		return fmt.Errorf("DeleteAttachments: %w", err)
	}

	for i := range rel.Attachments {
		attachment := rel.Attachments[i]
		if err := storage.Attachments.Delete(attachment.RelativePath()); err != nil {
			log.Error("Delete attachment %s of release %s failed: %v", attachment.UUID, rel.ID, err)
		}
	}

	notification.NotifyDeleteRelease(ctx, doer, rel)

	return nil
}
