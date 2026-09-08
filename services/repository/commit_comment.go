// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/services/gitdiff"
)

// ErrCommitCommentCoordinates is returned when the requested diff coordinate
// does not resolve to a real line of the commit.
var ErrCommitCommentCoordinates = errors.New("comment coordinates do not resolve to a line in this commit")

// ErrCommitCommentRootPrevious is returned when commenting on the old side of a
// root commit that has no parent.
var ErrCommitCommentRootPrevious = errors.New("cannot comment on the previous side of a root commit")

// CreateCommitComment stores an inline comment on a commit diff.
// line is signed: negative for the old side, positive for the new one.
// attachments are UUIDs of already-uploaded files to bind onto the Comment.
func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitSHA, treePath string, line int64, content string, attachments []string) (*issues_model.Comment, error) {
	commit, err := gitRepo.GetCommit(ctx, commitSHA)
	if err != nil {
		return nil, err
	}
	fullSHA := commit.ID.String()

	var parentSHA string
	if commit.ParentCount() > 0 {
		parentID, err := commit.ParentID(0)
		if err == nil {
			parentSHA = parentID.String()
		}
	}
	if parentSHA == "" && line < 0 {
		return nil, ErrCommitCommentRootPrevious
	}

	patch, err := commitCommentPatch(ctx, gitRepo, parentSHA, fullSHA, treePath, line)
	if err != nil {
		return nil, err
	}

	return issues_model.CreateCommitComment(ctx, &issues_model.CreateCommitCommentOptions{
		Repo:        repo,
		Doer:        doer,
		CommitSHA:   fullSHA,
		TreePath:    treePath,
		Line:        line,
		Content:     content,
		Patch:       patch,
		Attachments: attachments,
	})
}

func commitCommentPatch(ctx context.Context, gitRepo *git.Repository, parentSHA, fullSHA, treePath string, line int64) (string, error) {
	if parentSHA != "" {
		patch, err := git.GetFileDiffCutAroundLine(ctx, gitRepo, parentSHA, fullSHA, treePath, max(line, -line), line < 0, setting.UI.CodeCommentLines)
		if err != nil {
			log.Debug("GetFileDiffCutAroundLine failed for commit comment: %v", err)
		}
		if patch != "" {
			return patch, nil
		}
	}

	contextSHA := fullSHA
	if line < 0 {
		contextSHA = parentSHA
	}
	patch, err := gitdiff.GeneratePatchForUnchangedLine(ctx, gitRepo, contextSHA, treePath, line, setting.UI.CodeCommentLines)
	if err != nil {
		log.Debug("GeneratePatchForUnchangedLine failed for commit comment: %v", err)
	}
	if patch == "" {
		return "", ErrCommitCommentCoordinates
	}
	return patch, nil
}
