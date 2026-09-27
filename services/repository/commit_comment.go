// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"strings"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/util"
)

var (
	ErrCommitCommentLineInvalid = util.NewInvalidArgumentErrorf("commit comment line must be greater than zero")
	ErrCommitCommentEmpty       = util.NewInvalidArgumentErrorf("commit comment content must not be empty")
)

// CreateCommitComment creates an inline comment on a line of a commit. The commit and tree path are
// validated against the repository so a comment can not be attached to something that does not exist.
func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commitSHA, treePath string, line int64, content string) (*issues_model.Comment, error) {
	if line < 1 {
		return nil, ErrCommitCommentLineInvalid
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrCommitCommentEmpty
	}

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(ctx, commitSHA)
	if err != nil {
		return nil, err
	}
	if _, err := commit.GetTreeEntryByPath(ctx, gitRepo, treePath); err != nil {
		return nil, err
	}

	return issues_model.CreateCommitComment(ctx, &issues_model.CreateCommitCommentOptions{
		Doer:      doer,
		RepoID:    repo.ID,
		CommitSHA: commit.ID.String(),
		TreePath:  treePath,
		Line:      line,
		Content:   content,
	})
}
