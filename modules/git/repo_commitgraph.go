// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// WriteCommitGraph write commit graph to speed up repo access
// this requires git v2.18 to be installed
func WriteCommitGraph(ctx context.Context, repoPath string) error {
	if DefaultFeatures().CheckVersionAtLeast("2.18") {
		if _, _, err := gitcmd.NewCommand("commit-graph", "write").WithDir(repoPath).RunStdString(ctx); err != nil {
			return fmt.Errorf("unable to write commit-graph for '%s' : %w", repoPath, err)
		}
	}
	return nil
}
