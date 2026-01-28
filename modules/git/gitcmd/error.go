// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type RunStdError interface {
	error
	Unwrap() error
	Stderr() string
}

type runStdError struct {
	err    error  // usually the low-level error like `*exec.ExitError`
	stderr string // git command's stderr output
	errMsg string // the cached error message for Error() method
}

func (r *runStdError) Error() string {
	// FIXME: GIT-CMD-STDERR: it is a bad design, the stderr should not be put in the error message
	// But a lot of code only checks `strings.Contains(err.Error(), "git error")`
	if r.errMsg == "" {
		r.errMsg = fmt.Sprintf("%s - %s", r.err.Error(), strings.TrimSpace(r.stderr))
	}
	return r.errMsg
}

func (r *runStdError) Unwrap() error {
	return r.err
}

func (r *runStdError) Stderr() string {
	return r.stderr
}

func ErrorAsStderr(err error) (string, bool) {
	var runErr RunStdError
	if errors.As(err, &runErr) {
		return runErr.Stderr(), true
	}
	return "", false
}

func StderrHasPrefix(err error, prefix string) bool {
	stderr, ok := ErrorAsStderr(err)
	if !ok {
		return false
	}
	return strings.HasPrefix(stderr, prefix)
}

func IsErrorExitCode(err error, code int) bool {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode() == code
	}
	return false
}

func IsErrorSignalKilled(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.String() == "signal: killed"
}

func IsErrorCanceledOrKilled(err error) bool {
	// When "cancel()" a git command's context, the returned error of "Run()" could be one of them:
	// - context.Canceled
	// - *exec.ExitError: "signal: killed"
	// TODO: in the future, we need to use unified error type from gitcmd.Run to check whether it is manually canceled
	return errors.Is(err, context.Canceled) || IsErrorSignalKilled(err)
}

type pipelineError struct {
	error
}

func (e pipelineError) Unwrap() error {
	return e.error
}

func wrapPipelineError(err error) error {
	if err == nil {
		return nil
	}
	return pipelineError{err}
}

func UnwrapPipelineError(err error) (error, bool) { //nolint:revive // this is for error unwrapping
	var pe pipelineError
	if errors.As(err, &pe) {
		return pe.error, true
	}
	return nil, false
}
