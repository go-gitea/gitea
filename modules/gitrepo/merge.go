// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// MergeBase checks and returns merge base of two commits.
func MergeBase(ctx context.Context, repo Repository, baseCommitID, headCommitID string) (string, error) {
	mergeBase, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base").
		AddDashesAndList(baseCommitID, headCommitID))
	if err != nil {
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", baseCommitID, headCommitID, err)
	}
	return strings.TrimSpace(mergeBase), nil
}
