// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"
)

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// ListOccurrences lists all refs of the given refType the given commit appears in sorted by creation date DESC
// refType should only be a literal "branch" or "tag" and nothing else
func (repo *Repository) ListOccurrences(ctx context.Context, refType, commitSHA string) ([]string, error) {
	stdout, _, err := NewCommand(ctx, ToTrustedCmdArgs([]string{refType, "--no-color", "--sort=-creatordate", "--contains"})...).AddDynamicArguments(commitSHA).RunStdString(&RunOpts{Dir: repo.Path})

	refs := strings.Split(strings.TrimSpace(stdout), "\n")
	results := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref, "* ") { // main branch
			results = append(results, ref[len("* "):])
		} else if strings.HasPrefix(ref, "  ") { // all other branches
			results = append(results, ref[len("  "):])
		} else if ref != "" { // tags
			results = append(results, ref)
		}
	}
	return results, err
}
