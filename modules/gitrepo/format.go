// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// GetObjectFormatOfRepo returns the hash type of repository at a given path
func GetObjectFormatOfRepo(ctx context.Context, repo Repository) (git.ObjectFormat, error) {
	var stdout, stderr strings.Builder

	cmd := git.NewCommand(ctx, "hash-object", "--stdin")
	err := RunGitCmd(repo, cmd, &RunOpts{
		RunOpts: git.RunOpts{
			Stdout: &stdout,
			Stderr: &stderr,
			Stdin:  &strings.Reader{},
		},
	})
	if err != nil {
		return nil, err
	}

	if stderr.Len() > 0 {
		return nil, errors.New(stderr.String())
	}

	h, err := git.NewIDFromString(strings.TrimRight(stdout.String(), "\n"))
	if err != nil {
		return nil, err
	}

	return h.Type(), nil
}
