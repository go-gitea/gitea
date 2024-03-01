// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// ListOccurrences lists all refs of the given refType the given commit appears in sorted by creation date DESC
// refType should only be a literal "branch" or "tag" and nothing else
func (repo *Repository) ListOccurrences(ctx context.Context, refType, commitSHA string) ([]string, error) {
	cmd := NewCommand(ctx)
	if refType == "branch" {
		cmd.AddArguments("branch")
	} else if refType == "tag" {
		cmd.AddArguments("tag")
	} else {
		return nil, util.NewInvalidArgumentErrorf(`can only use "branch" or "tag" for refType, but got %q`, refType)
	}
	stdout, _, err := cmd.AddArguments("--no-color", "--sort=-creatordate", "--contains").AddDynamicArguments(commitSHA).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}

	refs := strings.Split(strings.TrimSpace(stdout), "\n")
	if refType == "branch" {
		return parseBranches(refs), nil
	}
	return parseTags(refs), nil
}

func parseBranches(refs []string) []string {
	results := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref, "* ") { // current branch (main branch)
			results = append(results, ref[len("* "):])
		} else if strings.HasPrefix(ref, "  ") { // all other branches
			results = append(results, ref[len("  "):])
		} else if ref != "" {
			results = append(results, ref)
		}
	}
	return results
}

func parseTags(refs []string) []string {
	results := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref != "" {
			results = append(results, ref)
		}
	}
	return results
}
