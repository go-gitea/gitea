// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CreateCommitCodeComment creates an inline comment on a specific line of a commit
func CreateCommitCodeComment(
	ctx context.Context,
	doer *user_model.User,
	repo *repo_model.Repository,
	gitRepo *git.Repository,
	commitSHA string,
	line int64,
	content string,
	treePath string,
	attachments []string,
) (*issues_model.Comment, error) {
	// Validate that the commit exists
	commit, err := gitRepo.GetCommit(commitSHA)
	if err != nil {
		log.Error("GetCommit failed: %v", err)
		return nil, err
	}

	// Create the comment using CreateCommentOptions
	comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:        issues_model.CommentTypeCommitCode,
		Doer:        doer,
		Repo:        repo,
		Content:     content,
		LineNum:     line,
		TreePath:    treePath,
		CommitSHA:   commit.ID.String(),
		Attachments: attachments,
	})
	if err != nil {
		log.Error("CreateComment failed: %v", err)
		return nil, err
	}

	// Load the poster for the comment
	if err = comment.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster failed: %v", err)
		return nil, err
	}

	// Load attachments
	if err = comment.LoadAttachments(ctx); err != nil {
		log.Error("LoadAttachments failed: %v", err)
		return nil, err
	}

	// Send notifications for mentions (pass nil for issue since this is a commit comment)
	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, nil, doer, comment.Content)
	if err != nil {
		log.Error("FindAndUpdateIssueMentions failed: %v", err)
	}

	// Notify about the new commit comment using CreateIssueComment
	// (commit comments use the same notification path as issue comments)
	notify_service.CreateIssueComment(ctx, doer, repo, nil, comment, mentions)

	return comment, nil
}

// UpdateCommitCodeComment updates an existing commit inline comment
func UpdateCommitCodeComment(
	ctx context.Context,
	doer *user_model.User,
	comment *issues_model.Comment,
	content string,
	attachments []string,
) error {
	// Verify the user has permission to edit
	if comment.PosterID != doer.ID {
		return util.ErrPermissionDenied
	}

	// Update content
	oldContent := comment.Content
	comment.Content = content

	if err := issues_model.UpdateComment(ctx, comment, comment.ContentVersion, doer); err != nil {
		comment.Content = oldContent
		return err
	}

	// Update attachments if provided
	if len(attachments) > 0 {
		if err := issues_model.UpdateCommentAttachments(ctx, comment, attachments); err != nil {
			return err
		}
	}

	return nil
}

// DeleteCommitCodeComment deletes a commit inline comment
func DeleteCommitCodeComment(
	ctx context.Context,
	doer *user_model.User,
	comment *issues_model.Comment,
) error {
	// Verify the user has permission to delete
	if comment.PosterID != doer.ID {
		return util.ErrPermissionDenied
	}

	return issues_model.DeleteComment(ctx, comment)
}
