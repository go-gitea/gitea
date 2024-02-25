// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

type RunOpts struct {
	git.RunOpts
	IsWiki bool
}

// RunGitCmdStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdString(repo Repository, c *git.Command, opts *RunOpts) (stdout, stderr string, runErr git.RunStdError) {
	stdoutBytes, stderrBytes, err := RunGitCmdStdBytes(repo, c, opts)
	stdout = util.UnsafeBytesToString(stdoutBytes)
	stderr = util.UnsafeBytesToString(stderrBytes)
	if err != nil {
		return stdout, stderr, git.NewRunStdError(err, stderr)
	}
	// even if there is no err, there could still be some stderr output, so we just return stdout/stderr as they are
	return stdout, stderr, nil
}

// RunGitCmdStdBytes runs the command with options and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdBytes(repo Repository, c *git.Command, opts *RunOpts) (stdout, stderr []byte, runErr git.RunStdError) {
	if opts == nil {
		opts = &RunOpts{}
	}
	if opts.Stdout != nil || opts.Stderr != nil {
		// we must panic here, otherwise there would be bugs if developers set Stdin/Stderr by mistake, and it would be very difficult to debug
		panic("stdout and stderr field must be nil when using RunStdBytes")
	}
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// We must not change the provided options as it could break future calls - therefore make a copy.
	newOpts := &RunOpts{
		RunOpts: git.RunOpts{
			Env:               opts.Env,
			Timeout:           opts.Timeout,
			UseContextTimeout: opts.UseContextTimeout,
			Dir:               opts.Dir,
			Stdout:            stdoutBuf,
			Stderr:            stderrBuf,
			Stdin:             opts.Stdin,
			PipelineFunc:      opts.PipelineFunc,
		},
		IsWiki: opts.IsWiki,
	}

	err := RunGitCmd(repo, c, newOpts)
	stderr = stderrBuf.Bytes()
	if err != nil {
		return nil, stderr, git.NewRunStdError(err, util.UnsafeBytesToString(stderr))
	}
	// even if there is no err, there could still be some stderr output
	return stdoutBuf.Bytes(), stderr, nil
}

// RunGitCmd runs the command with the RunOpts
func RunGitCmd(repo Repository, c *git.Command, opts *RunOpts) error {
	return curService.Run(repo, c, opts)
}
