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
	"gitea.dev/modules/git/gitcmd"
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

// ErrCherryPickConflict represents a cherry-pick that can not be applied cleanly
type ErrCherryPickConflict struct {
	Files []string
}

// IsErrCherryPickConflict checks if an error is an ErrCherryPickConflict.
func IsErrCherryPickConflict(err error) bool {
	var target *ErrCherryPickConflict
	return errors.As(err, &target)
}

func (err *ErrCherryPickConflict) Error() string {
	return "conflicts in: " + strings.Join(err.Files, ", ")
}

// ErrCherryPickNoChanges is returned when the result of a cherry-pick equals the target branch
var ErrCherryPickNoChanges = errors.New("the change is already applied, nothing to commit")

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
	conflict, conflictedFiles, err := pull.AttemptThreeWayMerge(ctx, t.basePath, t.gitRepo, base, opts.LastCommitID, right, description)
	if err != nil {
		return nil, fmt.Errorf("failed to three-way merge %s onto %s: %w", right, opts.OldBranch, err)
	}

	if conflict {
		return nil, &ErrCherryPickConflict{Files: conflictedFiles}
	}

	if opts.FailOnEmpty {
		err := gitcmd.NewCommand("diff-index", "--cached", "--quiet", "HEAD").WithRepo(t.gitRepo).RunWithStderr(ctx)
		if err == nil {
			return nil, ErrCherryPickNoChanges
		} else if !gitcmd.IsErrorExitCode(err, 1) {
			return nil, fmt.Errorf("CherryPick: diff-index: %w", err)
		}
	}

	if opts.DryRun {
		return &structs.FileResponse{}, nil
	}

	return gitPatchCommitPush(ctx, t, repo, gitRepo, doer, opts)
}
