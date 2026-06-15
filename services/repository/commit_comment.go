// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/services/gitdiff"
)

// CreateCommitComment creates an inline comment on a single commit at the given
// file path and line. The "line" follows the code-comment convention: a negative
// value points at the previous (old) side, a positive value at the proposed (new) side.
func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitID, content, treePath string, line int64, attachments []string) (*issues_model.Comment, error) {
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	commitID = commit.ID.String()

	var patch string
	if line != 0 && treePath != "" {
		var parentID string
		if commit.ParentCount() > 0 {
			parentSHA, err := commit.ParentID(0)
			if err != nil {
				return nil, err
			}
			parentID = parentSHA.String()
		}

		unsignedLine := int64((&issues_model.Comment{Line: line}).UnsignedLine())
		patch, err = git.GetFileDiffCutAroundLine(gitRepo, parentID, commitID, treePath, unsignedLine, line < 0, setting.UI.CodeCommentLines)
		if err != nil {
			return nil, err
		}
		// For a line that is unchanged by the commit the cut diff is empty, so fall
		// back to rendering the surrounding code context from the commit itself.
		if patch == "" {
			patch, err = gitdiff.GeneratePatchForUnchangedLine(gitRepo, commitID, treePath, line, setting.UI.CodeCommentLines)
			if err != nil {
				// the snippet is only decorative, so do not fail comment creation
				log.Debug("GeneratePatchForUnchangedLine (file=%s, line=%d, commit=%s): %v", treePath, line, commitID, err)
				patch = ""
			}
		}
	}

	return issues_model.CreateCommitComment(ctx, &issues_model.CreateCommitCommentOptions{
		Doer:        doer,
		Repo:        repo,
		CommitSHA:   commitID,
		Content:     content,
		TreePath:    treePath,
		LineNum:     line,
		Patch:       patch,
		Attachments: attachments,
	})
}
