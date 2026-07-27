// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"errors"
	"fmt"
	"strings"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/structs"
	"gitea.dev/services/pull"
)

// ErrCommitIDDoesNotMatch represents a "CommitIDDoesNotMatch" kind of error.
type ErrCommitIDDoesNotMatch struct {
	GivenCommitID   string
	CurrentCommitID string
}

// IsErrCommitIDDoesNotMatch checks if an error is a ErrCommitIDDoesNotMatch.
func IsErrCommitIDDoesNotMatch(err error) bool {
	_, ok := err.(ErrCommitIDDoesNotMatch)
	return ok
}

func (err ErrCommitIDDoesNotMatch) Error() string {
	return fmt.Sprintf("file CommitID does not match [given: %s, expected: %s]", err.GivenCommitID, err.CurrentCommitID)
}

// CherryPick cherry-picks or reverts a commit to the given repository
func CherryPick(ctx reqctx.RequestContext, repo *repo_model.Repository, doer *user_model.User, revert bool, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	t, err := gitPatchPrepare(ctx, repo, gitRepo, doer, opts)
	if err != nil {
		return nil, err
	}
	defer t.Close()

	err = t.RefreshIndex(ctx)
	if err != nil {
		return nil, err
	}

	commit, err := t.GetCommit(ctx, strings.TrimSpace(opts.Content))
	if err != nil {
		return nil, err
	}
	parent, err := commit.ParentID(0)
	if err != nil {
		parent = git.ObjectFormatFromName(repo.ObjectFormatName).EmptyTree()
	}

	base, right := parent.String(), commit.ID.String()

	if revert {
		right, base = base, right
	}

	description := fmt.Sprintf("CherryPick %s onto %s", right, opts.OldBranch)
	conflict, _, err := pull.AttemptThreeWayMerge(ctx, t.basePath, t.gitRepo, base, opts.LastCommitID, right, description)
	if err != nil {
		return nil, fmt.Errorf("failed to three-way merge %s onto %s: %w", right, opts.OldBranch, err)
	}

	if conflict {
		return nil, errors.New("failed to merge due to conflicts")
	}

	return gitPatchCommitPush(ctx, t, repo, gitRepo, doer, opts)
}
