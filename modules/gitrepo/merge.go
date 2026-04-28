// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

type ErrNoMergeBase struct {
	BaseCommitID string
	HeadCommitID string
	Err          error
}

func (err ErrNoMergeBase) Error() string {
	return fmt.Sprintf("get merge-base of %s and %s failed: %v", err.BaseCommitID, err.HeadCommitID, err.Err)
}

func (err ErrNoMergeBase) Unwrap() error {
	return err.Err
}

// MergeBase checks and returns merge base of two commits.
func MergeBase(ctx context.Context, repo Repository, baseCommitID, headCommitID string) (string, error) {
	mergeBase, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base").
		AddDashesAndList(baseCommitID, headCommitID))
	if err != nil {
		if gitcmd.IsErrorExitCode(err, 1) {
			return "", ErrNoMergeBase{
				BaseCommitID: baseCommitID,
				HeadCommitID: headCommitID,
				Err:          err,
			}
		}
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", baseCommitID, headCommitID, err)
	}
	return strings.TrimSpace(mergeBase), nil
}
