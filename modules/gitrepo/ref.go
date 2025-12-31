// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
)

func UpdateRef(ctx context.Context, repo Repository, refName, newCommitID string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("update-ref").AddDynamicArguments(refName, newCommitID))
}

func RemoveRef(ctx context.Context, repo Repository, refName string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(refName))
}

// ListOccurrences lists all refs of the given refType the given commit appears in sorted by creation date DESC
// refType should only be a literal "branch" or "tag" and nothing else
func ListOccurrences(ctx context.Context, repo Repository, refType, commitSHA string) ([]string, error) {
	cmd := gitcmd.NewCommand()
	switch refType {
	case "branch":
		cmd.AddArguments("branch")
	case "tag":
		cmd.AddArguments("tag")
	default:
		return nil, util.NewInvalidArgumentErrorf(`can only use "branch" or "tag" for refType, but got %q`, refType)
	}
	stdout, err := RunCmdString(ctx, repo, cmd.AddArguments("--no-color", "--sort=-creatordate", "--contains").
		AddDynamicArguments(commitSHA))
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
