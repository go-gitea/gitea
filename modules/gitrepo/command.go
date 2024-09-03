// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

// RunGitCmd runs the command with the RunOpts
func RunGitCmd(ctx context.Context, repo Repository, c *git.Command, opts *git.RunOpts) error {
	if opts.Dir != "" {
		return fmt.Errorf("dir field must be empty")
	}
	opts.Dir = repoRelativePath(repo)
	return curService.Run(ctx, c, opts)
}

type runStdError struct {
	err    error
	stderr string
	errMsg string
}

func (r *runStdError) Error() string {
	// the stderr must be in the returned error text, some code only checks `strings.Contains(err.Error(), "git error")`
	if r.errMsg == "" {
		r.errMsg = git.ConcatenateError(r.err, r.stderr).Error()
	}
	return r.errMsg
}

func (r *runStdError) Unwrap() error {
	return r.err
}

func (r *runStdError) Stderr() string {
	return r.stderr
}

// RunStdBytes runs the command with options and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdBytes(ctx context.Context, repo Repository, c *git.Command, opts *git.RunOpts) (stdout, stderr []byte, runErr git.RunStdError) {
	if opts == nil {
		opts = &git.RunOpts{}
	}
	if opts.Stdout != nil || opts.Stderr != nil {
		// we must panic here, otherwise there would be bugs if developers set Stdin/Stderr by mistake, and it would be very difficult to debug
		panic("stdout and stderr field must be nil when using RunStdBytes")
	}
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// We must not change the provided options as it could break future calls - therefore make a copy.
	newOpts := &git.RunOpts{
		Env:               opts.Env,
		Timeout:           opts.Timeout,
		UseContextTimeout: opts.UseContextTimeout,
		Stdout:            stdoutBuf,
		Stderr:            stderrBuf,
		Stdin:             opts.Stdin,
		PipelineFunc:      opts.PipelineFunc,
	}

	err := RunGitCmd(ctx, repo, c, newOpts)
	stderr = stderrBuf.Bytes()
	if err != nil {
		return nil, stderr, &runStdError{err: err, stderr: util.UnsafeBytesToString(stderr)}
	}
	// even if there is no err, there could still be some stderr output
	return stdoutBuf.Bytes(), stderr, nil
}

// RunStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func RunGitCmdStdString(ctx context.Context, repo Repository, c *git.Command, opts *git.RunOpts) (stdout, stderr string, runErr git.RunStdError) {
	stdoutBytes, stderrBytes, err := RunGitCmdStdBytes(ctx, repo, c, opts)
	stdout = util.UnsafeBytesToString(stdoutBytes)
	stderr = util.UnsafeBytesToString(stderrBytes)
	if err != nil {
		return stdout, stderr, &runStdError{err: err, stderr: stderr}
	}
	// even if there is no err, there could still be some stderr output, so we just return stdout/stderr as they are
	return stdout, stderr, nil
}
