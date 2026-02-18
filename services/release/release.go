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
	"code.gitea.io/gitea/modules/git/gitcmd"
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

// createGitTag creates a git tag for the release, if the tag already exists, it will return an error.
// sha1 should not be empty, and it should be a valid commit SHA, otherwise it will return an error.
func createGitTag(ctx context.Context, gitRepo *git.Repository, repoID, publisherID int64, tagName, sha1, msg string) error {
	if sha1 == "" {
		return fmt.Errorf("cannot create tag: target commit sha1 is empty (tag=%s)", tagName)
	}
	protectedTags, err := git_model.GetProtectedTags(ctx, repoID)
	if err != nil {
		return fmt.Errorf("GetProtectedTags: %w", err)
	}

	if isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, protectedTags, tagName, publisherID); err != nil {
		return err
	} else if !isAllowed {
		return ErrProtectedTagName{
			TagName: tagName,
		}
	}

	if len(msg) > 0 {
		err = gitRepo.CreateAnnotatedTag(tagName, msg, sha1)
	} else {
		err = gitRepo.CreateTag(tagName, sha1)
	}

	switch {
	case strings.Contains(err.Error(), "is not a valid tag name"):
		return ErrInvalidTagName{
			TagName: tagName,
		}
	case strings.Contains(err.Error(), "already exists"):
		return ErrTagAlreadyExists{
			TagName: tagName,
		}
	default:
		return err
	}
}

// CreateRelease creates a new release of repository.
func CreateRelease(ctx context.Context, gitRepo *git.Repository, rel *repo_model.Release, attachmentUUIDs []string, msg string) error {
	if rel.Repo == nil || rel.Publisher == nil {
		return errors.New("repo or publisher is not loaded")
	}
	if err := rel.Repo.MustNotBeArchived(); err != nil {
		return err
	}

	has, err := repo_model.IsReleaseExist(ctx, rel.RepoID, rel.TagName)
	if err != nil {
		return err
	} else if has {
		return repo_model.ErrReleaseAlreadyExist{
			TagName: rel.TagName,
		}
	}

	var commit *git.Commit
	var needsCreateTag bool
	if !rel.IsDraft {
		if !gitrepo.IsTagExist(ctx, rel.Repo, rel.TagName) {
			commit, err = gitRepo.GetCommit(rel.Target)
			if err != nil {
				return err
			}
			needsCreateTag = true
		} else {
			commit, err = gitRepo.GetTagCommit(rel.TagName)
			if err != nil {
				return err
			}
			needsCreateTag = false
		}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		rel.CreatedUnix = timeutil.TimeStampNow()
		rel.Title = util.EllipsisDisplayString(rel.Title, 255)
		rel.LowerTagName = strings.ToLower(rel.TagName)
		if commit != nil {
			rel.Sha1 = commit.ID.String()
			rel.NumCommits, err = gitrepo.CommitsCountOfCommit(ctx, rel.Repo, commit.ID.String())
			if err != nil {
				return fmt.Errorf("CommitsCount: %w", err)
			}
			if rel.PublisherID <= 0 {
				u, err := user_model.GetUserByEmail(ctx, commit.Author.Email)
				if err == nil {
					rel.PublisherID = u.ID
				}
			}
		}

		if err = db.Insert(ctx, rel); err != nil {
			return err
		}

		if err = repo_model.AddReleaseAttachments(ctx, rel.ID, attachmentUUIDs); err != nil {
			return err
		}

		if needsCreateTag {
			err = createGitTag(ctx, gitRepo, rel.RepoID, rel.PublisherID, rel.TagName, rel.Sha1, msg)
		}
		return err
	}); err != nil {
		return err
	}

	if !rel.IsDraft {
		if needsCreateTag {
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
		}
		notify_service.NewRelease(ctx, rel)
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
func CreateNewTag(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, targetBranchOrCommit, tagName, msg string) error {
	if err := repo.MustNotBeArchived(); err != nil {
		return err
	}

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

	commit, err := gitRepo.GetCommit(targetBranchOrCommit)
	if err != nil {
		return err
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		rel := &repo_model.Release{
			RepoID:       repo.ID,
			Repo:         repo,
			PublisherID:  doer.ID,
			Publisher:    doer,
			TagName:      tagName,
			LowerTagName: strings.ToLower(tagName),
			Target:       targetBranchOrCommit,
			IsDraft:      false,
			IsPrerelease: false,
			IsTag:        true,
			CreatedUnix:  timeutil.TimeStampNow(),
			Sha1:         commit.ID.String(),
		}

		if err := db.Insert(ctx, rel); err != nil {
			return err
		}

		return createGitTag(ctx, gitRepo, rel.RepoID, rel.PublisherID, rel.TagName, rel.Sha1, msg)
	}); err != nil {
		return err
	}

	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	commits := repository.NewPushCommits()
	commits.HeadCommit = repository.CommitToPushCommit(commit)
	commits.CompareURL = repo.ComposeCompareURL(objectFormat.EmptyObjectID().String(), commit.ID.String())

	refFullName := git.RefNameFromTag(tagName)
	notify_service.PushCommits(
		ctx, doer, repo,
		&repository.PushUpdateOptions{
			RefFullName: refFullName,
			OldCommitID: objectFormat.EmptyObjectID().String(),
			NewCommitID: commit.ID.String(),
		}, commits)
	notify_service.CreateRef(ctx, doer, repo, refFullName, commit.ID.String())

	return nil
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

	oldRelease, err := repo_model.GetReleaseByID(ctx, rel.ID)
	if err != nil {
		return err
	}
	if err := oldRelease.LoadAttributes(ctx); err != nil {
		return err
	}
	if err := oldRelease.Repo.MustNotBeArchived(); err != nil {
		return err
	}

	rel.Repo = oldRelease.Repo
	isConvertFromDraft := oldRelease.IsDraft && !rel.IsDraft
	isConvertedFromTag := oldRelease.IsTag && !rel.IsTag
	needsCreateTag := false
	var commit *git.Commit
	if !rel.IsDraft {
		if !gitrepo.IsTagExist(ctx, rel.Repo, rel.TagName) {
			commit, err = gitRepo.GetCommit(rel.Target)
			if err != nil {
				return err
			}

			rel.Sha1 = commit.ID.String()
			rel.NumCommits, err = gitrepo.CommitsCountOfCommit(ctx, rel.Repo, commit.ID.String())
			if err != nil {
				return fmt.Errorf("CommitsCount: %w", err)
			}
			needsCreateTag = true
		} else { // validate the git tag exists
			_, err = gitRepo.GetTagCommit(rel.TagName)
			if err != nil {
				return err
			}
		}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		rel.LowerTagName = strings.ToLower(rel.TagName)
		if oldRelease.IsDraft { // if it's a draft release, we should update CreatedUnix to current time
			rel.CreatedUnix = timeutil.TimeStampNow()
		}
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

		if needsCreateTag {
			return createGitTag(ctx, gitRepo, rel.RepoID, rel.PublisherID, rel.TagName, rel.Sha1, "")
		}

		return nil
	}); err != nil {
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
		if needsCreateTag {
			objectFormat := git.ObjectFormatFromName(rel.Repo.ObjectFormatName)
			commits := repository.NewPushCommits()
			commits.HeadCommit = repository.CommitToPushCommit(commit)
			commits.CompareURL = rel.Repo.ComposeCompareURL(objectFormat.EmptyObjectID().String(), commit.ID.String())

			refFullName := git.RefNameFromTag(rel.TagName)
			notify_service.PushCommits(
				ctx, doer, rel.Repo,
				&repository.PushUpdateOptions{
					RefFullName: refFullName,
					OldCommitID: objectFormat.EmptyObjectID().String(),
					NewCommitID: commit.ID.String(),
				}, commits)
			notify_service.CreateRef(ctx, doer, rel.Repo, refFullName, commit.ID.String())
		}
		if !isConvertFromDraft && !isConvertedFromTag {
			notify_service.UpdateRelease(ctx, doer, rel)
		} else {
			notify_service.NewRelease(ctx, rel)
		}
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
		isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, protectedTags, rel.TagName, doer.ID)
		if err != nil {
			return err
		}
		if !isAllowed {
			return ErrProtectedTagName{
				TagName: rel.TagName,
			}
		}

		if stdout, _, err := gitrepo.RunCmdString(ctx, repo,
			gitcmd.NewCommand("tag", "-d").AddDashesAndList(rel.TagName),
		); err != nil && !strings.Contains(err.Error(), "not found") {
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
