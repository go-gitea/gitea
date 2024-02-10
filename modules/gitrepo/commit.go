// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// AllCommitsCount returns count of all commits in repository
func AllCommitsCount(ctx context.Context, repo Repository, hidePRRefs bool, files ...string) (int64, error) {
	cmd := git.NewCommand(ctx, "rev-list")
	if hidePRRefs {
		cmd.AddArguments("--exclude=" + git.PullPrefix + "*")
	}
	cmd.AddArguments("--all", "--count")
	if len(files) > 0 {
		cmd.AddDashesAndList(files...)
	}

	stdout, _, err := RunGitCmdStdString(repo, cmd, &RunOpts{})
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}
