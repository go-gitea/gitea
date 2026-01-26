// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetDiff generates and returns patch data between given revisions, optimized for human readability
func GetDiff(ctx context.Context, repo Repository, compareArg string, w io.Writer) error {
	return RunCmdWithStderr(ctx, repo, gitcmd.NewCommand("diff", "-p").AddDynamicArguments(compareArg).
		WithStdoutCopy(w))
}

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func GetDiffBinary(ctx context.Context, repo Repository, compareArg string, w io.Writer) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("diff", "-p", "--binary", "--histogram").
		AddDynamicArguments(compareArg).
		WithStdoutCopy(w))
}

// GetPatch generates and returns format-patch data between given revisions, able to be used with `git apply`
func GetPatch(ctx context.Context, repo Repository, compareArg string, w io.Writer) error {
	return RunCmdWithStderr(ctx, repo, gitcmd.NewCommand("format-patch", "--binary", "--stdout").AddDynamicArguments(compareArg).
		WithStdoutCopy(w))
}
