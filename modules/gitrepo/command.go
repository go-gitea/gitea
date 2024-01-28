// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"code.gitea.io/gitea/modules/git"
)

type RunOpts struct {
	git.RunOpts
	IsWiki bool
}

// RunGitCmdStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdString(repo Repository, c *git.Command, opts *RunOpts) (stdout, stderr string, runErr git.RunStdError) {
	if opts.Dir != "" {
		// we must panic here, otherwise there would be bugs if developers set Dir by mistake, and it would be very difficult to debug
		panic("dir field must be empty when using RunStdBytes")
	}
	opts.Dir = getPath(repo, opts.IsWiki)
	return c.RunStdString(&opts.RunOpts)
}

// RunGitCmdStdBytes runs the command with options and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdBytes(repo Repository, c *git.Command, opts *RunOpts) (stdout, stderr []byte, runErr git.RunStdError) {
	if opts.Dir != "" {
		// we must panic here, otherwise there would be bugs if developers set Dir by mistake, and it would be very difficult to debug
		panic("dir field must be empty when using RunStdBytes")
	}
	opts.Dir = getPath(repo, opts.IsWiki)
	return c.RunStdBytes(&opts.RunOpts)
}

// RunGitCmd runs the command with the RunOpts
func RunGitCmd(repo Repository, c *git.Command, opts *RunOpts) error {
	if opts.Dir != "" {
		// we must panic here, otherwise there would be bugs if developers set Dir by mistake, and it would be very difficult to debug
		panic("dir field must be empty when using RunStdBytes")
	}
	opts.Dir = getPath(repo, opts.IsWiki)
	return c.Run(&opts.RunOpts)
}
