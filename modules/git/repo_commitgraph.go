// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
)

// WriteCommitGraph write commit graph to speed up repo access
// this requires git v2.18 to be installed
func WriteCommitGraph(ctx context.Context, repoPath string) error {
	if CheckGitVersionAtLeast("2.18") == nil {
		if _, _, err := NewCommand(ctx, "commit-graph", "write").RunStdString(&RunOpts{Dir: repoPath}); err != nil {
			return fmt.Errorf("unable to write commit-graph for '%s' : %w", repoPath, err)
		}
	}
	return nil
}
