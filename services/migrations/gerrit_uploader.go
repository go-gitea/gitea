// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	base "gitea.dev/modules/migration"
)

// verifyGerritChange fails if the cloned repository lacks the target branch or the ref of a patch set of the change,
// a pull request on top of an incomplete clone would show a wrong diff
func (g *GiteaLocalUploader) verifyGerritChange(ctx context.Context, pr *base.PullRequest) error {
	change, ok := pr.Context.(*gerritChange)
	if !ok {
		return nil
	}
	if !g.gitRepo.IsBranchExist(ctx, pr.Base.Ref) {
		return fmt.Errorf("the target branch %q of gerrit change %d does not exist in the migrated repository", pr.Base.Ref, pr.Number)
	}
	for _, patchSet := range change.patchSets {
		stdout, _, err := gitcmd.NewCommand("rev-parse", "--verify", "--quiet").
			AddDynamicArguments(patchSet.Ref + "^{commit}").WithRepo(g.repo).RunStdString(ctx)
		if err != nil || strings.TrimSpace(stdout) != patchSet.SHA {
			return fmt.Errorf("the ref %s of patch set %d of gerrit change %d is missing or does not point to revision %s", patchSet.Ref, patchSet.Number, pr.Number, patchSet.SHA)
		}
	}
	if pr.Base.SHA != "" && !g.commitExists(ctx, pr.Base.SHA) {
		pr.Base.SHA = ""
	}
	return nil
}
