// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

// ErrInvalidTagName represents a "InvalidTagName" kind of error.
type ErrInvalidTagName struct {
	TagName string
}

// IsErrInvalidTagName checks if an error is a ErrInvalidTagName.
func IsErrInvalidTagName(err error) bool {
	_, ok := err.(ErrInvalidTagName)
	return ok
}

func (err ErrInvalidTagName) Error() string {
	return fmt.Sprintf("release tag name is not valid [tag_name: %s]", err.TagName)
}

func (err ErrInvalidTagName) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrProtectedTagName represents a "ProtectedTagName" kind of error.
type ErrProtectedTagName struct {
	TagName string
}

// IsErrProtectedTagName checks if an error is a ErrProtectedTagName.
func IsErrProtectedTagName(err error) bool {
	_, ok := err.(ErrProtectedTagName)
	return ok
}

func (err ErrProtectedTagName) Error() string {
	return fmt.Sprintf("release tag name is protected [tag_name: %s]", err.TagName)
}

func (err ErrProtectedTagName) Unwrap() error {
	return util.ErrPermissionDenied
}

func createTag(ctx context.Context, gitRepo *git.Repository, rel *repo_model.Release, msg string) (bool, error) {
	err := rel.LoadAttributes(ctx)
	if err != nil {
		return false, err
	}

	err = rel.Repo.MustNotBeArchived()
	if err != nil {
		return false, err
	}

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
				return false, ErrProtectedTagName{
					TagName: rel.TagName,
				}
			}

			commit, err := gitRepo.GetCommit(rel.Target)
			if err != nil {
				return false, err
			}

			if len(msg) > 0 {
				if err = gitRepo.CreateAnnotatedTag(rel.TagName, msg, commit.ID.String()); err != nil {
					if strings.Contains(err.Error(), "is not a valid tag name") {
						return false, ErrInvalidTagName{
							TagName: rel.TagName,
						}
					}
					return false, err
				}
			} else if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return false, ErrInvalidTagName{
						TagName: rel.TagName,
					}
				}
				return false, err
			}
			created = true
			rel.LowerTagName = strings.ToLower(rel.TagName)

			objectFormat := git.ObjectFormatFromName(rel.Repo.ObjectFormatName)
			commits := repository.NewPushCommits()
			commits.HeadCommit = repository.CommitToPushCommit(commit)
			commits.CompareURL = rel.Repo.ComposeCompareURL(objectFormat.EmptyObjectID().String(), commit.ID.String())

			refFullName := git.RefNameFromTag(rel.TagName)
			notify_service.PushCommits(
				ctx, rel.Publisher, rel.Repo,
				&repository.PushUpdateOptions{
					RefFullName: refFullName,
					OldCommitID: objectFormat.EmptyObjectID().String(),
					NewCommitID: commit.ID.String(),
				}, commits)
			notify_service.CreateRef(ctx, rel.Publisher, rel.Repo, refFullName, commit.ID.String())
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

	rel.Title = util.EllipsisDisplayString(rel.Title, 255)
	rel.LowerTagName = strings.ToLower(rel.TagName)
	if err = db.Insert(gitRepo.Ctx, rel); err != nil {
		return err
	}

	if err = repo_model.AddReleaseAttachments(gitRepo.Ctx, rel.ID, attachmentUUIDs); err != nil {
		return err
	}

	if !rel.IsDraft {
		notify_service.NewRelease(gitRepo.Ctx, rel)
	}

	return nil
}

// ErrTagAlreadyExists represents an error that tag with such name already exists.
type ErrTagAlreadyExists struct {
	TagName string
}

// IsErrTagAlreadyExists checks if an error is an ErrTagAlreadyExists.
func IsErrTagAlreadyExists(err error) bool {
	_, ok := err.(ErrTagAlreadyExists)
	return ok
}

func (err ErrTagAlreadyExists) Error() string {
	return fmt.Sprintf("tag already exists [name: %s]", err.TagName)
}

func (err ErrTagAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// CreateNewTag creates a new repository tag
func CreateNewTag(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commit, tagName, msg string) error {
	has, err := repo_model.IsReleaseExist(ctx, repo.ID, tagName)
	if err != nil {
		return err
	} else if has {
		return ErrTagAlreadyExists{
			TagName: tagName,
		}
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
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
func UpdateRelease(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, rel *repo_model.Release,
	addAttachmentUUIDs, delAttachmentUUIDs []string, editAttachments map[string]string,
) error {
	if rel.ID == 0 {
		return errors.New("UpdateRelease only accepts an exist release")
	}
	isTagCreated, err := createTag(gitRepo.Ctx, gitRepo, rel, "")
	if err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	oldRelease, err := repo_model.GetReleaseByID(ctx, rel.ID)
	if err != nil {
		return err
	}
	isConvertedFromTag := oldRelease.IsTag && !rel.IsTag

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
				return util.NewPermissionDeniedErrorf("delete attachment of release permission denied")
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
				return util.NewPermissionDeniedErrorf("update attachment of release permission denied")
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

	if err := committer.Commit(); err != nil {
		return err
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

	if !rel.IsDraft {
		if !isTagCreated && !isConvertedFromTag {
			notify_service.UpdateRelease(gitRepo.Ctx, doer, rel)
			return nil
		}
		notify_service.NewRelease(gitRepo.Ctx, rel)
	}
	return nil
}

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(ctx context.Context, repo *repo_model.Repository, rel *repo_model.Release, doer *user_model.User, delTag bool) error {
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
			return ErrProtectedTagName{
				TagName: rel.TagName,
			}
		}

		if stdout, _, err := git.NewCommand("tag", "-d").AddDashesAndList(rel.TagName).
			RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()}); err != nil && !strings.Contains(err.Error(), "not found") {
			log.Error("DeleteReleaseByID (git tag -d): %d in %v Failed:\nStdout: %s\nError: %v", rel.ID, repo, stdout, err)
			return fmt.Errorf("git tag -d: %w", err)
		}

		refName := git.RefNameFromTag(rel.TagName)
		objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
		notify_service.PushCommits(
			ctx, doer, repo,
			&repository.PushUpdateOptions{
				RefFullName: refName,
				OldCommitID: rel.Sha1,
				NewCommitID: objectFormat.EmptyObjectID().String(),
			}, repository.NewPushCommits())
		notify_service.DeleteRef(ctx, doer, repo, refName)

		if _, err := db.DeleteByID[repo_model.Release](ctx, rel.ID); err != nil {
			return fmt.Errorf("DeleteReleaseByID: %w", err)
		}
	} else {
		rel.IsTag = true

		if err := repo_model.UpdateRelease(ctx, rel); err != nil {
			return fmt.Errorf("Update: %w", err)
		}
	}

	rel.Repo = repo
	if err := rel.LoadAttributes(ctx); err != nil {
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

	if !rel.IsDraft {
		notify_service.DeleteRelease(ctx, doer, rel)
	}
	return nil
}

// Init start release service
func Init() error {
	return initTagSyncQueue(graceful.GetManager().ShutdownContext())
}
