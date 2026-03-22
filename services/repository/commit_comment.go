// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/gitdiff"
)

// CreateCommitCodeComment creates an inline comment on a commit diff line
func CreateCommitCodeComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitSHA, treePath, content string, line int64) (*repo_model.CommitCodeComment, error) {
	// Generate code context patch around the commented line
	var patch string

	commit, err := gitRepo.GetCommit(commitSHA)
	if err != nil {
		return nil, fmt.Errorf("GetCommit: %w", err)
	}

	// Get the parent commit for diff context
	var parentCommitID string
	if commit.ParentCount() > 0 {
		parentID, err := commit.ParentID(0)
		if err != nil {
			return nil, fmt.Errorf("ParentID: %w", err)
		}
		parentCommitID = parentID.String()
	}

	if parentCommitID != "" {
		unsignedLine := line
		if unsignedLine < 0 {
			unsignedLine = -unsignedLine
		}
		patch, err = git.GetFileDiffCutAroundLine(
			gitRepo, parentCommitID, commitSHA, treePath,
			unsignedLine, line < 0, setting.UI.CodeCommentLines,
		)
		if err != nil {
			log.Debug("GetFileDiffCutAroundLine: %v", err)
		}
	}

	// If patch is still empty, try generating for unchanged line
	if patch == "" {
		patch, err = gitdiff.GeneratePatchForUnchangedLine(gitRepo, commitSHA, treePath, line, setting.UI.CodeCommentLines)
		if err != nil {
			log.Debug("GeneratePatchForUnchangedLine: %v", err)
		}
	}

	return repo_model.CreateCommitCodeComment(ctx, &repo_model.CreateCommitCodeCommentOptions{
		Repo:      repo,
		Doer:      doer,
		CommitSHA: commitSHA,
		TreePath:  treePath,
		Line:      line,
		Content:   content,
		Patch:     patch,
	})
}
