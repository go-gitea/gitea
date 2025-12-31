// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// LsTree checks if the given filenames are in the tree
func LsTree(ctx context.Context, repo Repository, ref string, filenames ...string) ([]string, error) {
	res, _, err := RunCmdBytes(ctx, repo, gitcmd.NewCommand("ls-tree", "-z", "--name-only").
		AddDashesAndList(append([]string{ref}, filenames...)...))
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for line := range bytes.SplitSeq(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

// GetTreePathLatestCommitID returns the latest commit of a tree path
func GetTreePathLatestCommitID(ctx context.Context, repo Repository, refName, treePath string) (string, error) {
	stdout, err := RunCmdString(ctx, repo, gitcmd.NewCommand("rev-list", "-1").
		AddDynamicArguments(refName).AddDashesAndList(treePath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
