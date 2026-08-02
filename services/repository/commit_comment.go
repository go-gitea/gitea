// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/references"
	"gitea.dev/modules/setting"
	"gitea.dev/services/gitdiff"
)

// ErrCommitCommentCoordinates is returned when the requested diff coordinate
// does not resolve to a real line of the commit.
var ErrCommitCommentCoordinates = errors.New("comment coordinates do not resolve to a line in this commit")

// ErrCommitCommentRootPrevious is returned when commenting on the old side of a
// commit that has no parent, and therefore no old side.
var ErrCommitCommentRootPrevious = errors.New("cannot comment on the previous side of a root commit")

// CreateCommitComment stores an inline comment on a commit diff. line is signed:
// negative for the old side, positive for the new one.
func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitSHA, treePath string, line int64, content string) (*repo_model.CommitComment, error) {
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

	comment := &repo_model.CommitComment{
		RepoID:    repo.ID,
		CommitSHA: fullSHA,
		TreePath:  treePath,
		Line:      line,
		PosterID:  doer.ID,
		Poster:    doer,
		Content:   content,
		Patch:     patch,
	}
	if err := repo_model.CreateCommitComment(ctx, comment); err != nil {
		return nil, err
	}

	mentions := references.FindAllMentionsMarkdown(content)
	if err := activities_model.CreateCommitCommentNotification(ctx, doer, repo, fullSHA, comment.ID, commit.Author.Email, mentions); err != nil {
		log.Error("CreateCommitCommentNotification: %v", err)
	}
	return comment, nil
}

// commitCommentPatch builds the diff context stored with a comment. An empty
// patch means the coordinate does not exist, so it doubles as the validation
// that keeps a crafted POST from creating an invisible comment.
func commitCommentPatch(ctx context.Context, gitRepo *git.Repository, parentSHA, fullSHA, treePath string, line int64) (string, error) {
	if parentSHA != "" {
		absLine, isOld := line, line < 0
		if isOld {
			absLine = -line
		}
		patch, err := git.GetFileDiffCutAroundLine(ctx, gitRepo, parentSHA, fullSHA, treePath, absLine, isOld, setting.UI.CodeCommentLines)
		if err != nil {
			log.Debug("GetFileDiffCutAroundLine failed for commit comment: %v", err)
		}
		if patch != "" {
			return patch, nil
		}
	}

	// The line is unchanged by the commit (or the commit is a root commit).
	// Old-side coordinates only exist in the parent tree; resolving them against
	// fullSHA rejects lines the commit removed and stores the wrong context for
	// the ones it kept.
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
