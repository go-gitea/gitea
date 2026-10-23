// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"gitea.dev/modules/git/gitcmd"
)

// WriteCommitGraph write commit graph to speed up repo access
func WriteCommitGraph(ctx context.Context, repo RepositoryFacade) error {
	if _, _, err := gitcmd.NewCommand("commit-graph", "write").WithRepo(repo).RunStdString(ctx); err != nil {
		return fmt.Errorf("unable to write commit-graph for '%s' : %w", repo.GitRepoLocation(), err)
	}
	return nil
}
