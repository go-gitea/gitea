// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
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
	if runErr, ok := errors.AsType[RunStdError](err); ok {
		return runErr.Stderr(), true
	}
	return "", false
}

func IsErrorExitCode(err error, code int) bool {
	if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
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

type (
	StderrPrefix   string
	StderrWildcard string
)

const (
	StderrNotValidObjectName StderrPrefix = "fatal: not a valid object name"
	StderrNotTreeObject      StderrPrefix = "fatal: not a tree object"
	StderrPathSpec           StderrPrefix = "fatal: pathspec"
	StderrBadRevision        StderrPrefix = "fatal: bad revision"

	StderrNoSuchRemote1 StderrPrefix = "fatal: no such remote" // git < 2.30, exit status 128
	StderrNoSuchRemote2 StderrPrefix = "error: no such remote" // git >= 2.30. exit status 2

	StderrUnknownRevisionOrPath StderrWildcard = "fatal: *: unknown revision or path not in the working tree"
	StderrNoMergeBase           StderrWildcard = "fatal: *: no merge base"
)

func IsStderr[T StderrPrefix | StderrWildcard](err error, check T) bool {
	stderr, ok := ErrorAsStderr(err)
	if !ok {
		return false
	}
	checkLen := len(check)
	if len(stderr) < checkLen {
		return false
	}
	switch any(check).(type) {
	case StderrPrefix:
		// Git is lowercasing the "fatal: Not a valid object name" error message
		// ref: https://lore.kernel.org/git/pull.2052.git.1771836302101.gitgitgadget@gmail.com
		return util.AsciiEqualFold(stderr[:checkLen], string(check))
	case StderrWildcard:
		prefix, remaining, _ := strings.Cut(string(check), "*")
		return strings.HasPrefix(stderr, prefix) && strings.Contains(stderr, remaining)
	}
	setting.PanicInDevOrTesting("invalid stderr type %T", check)
	return false
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
	if pe, ok := errors.AsType[pipelineError](err); ok {
		return pe.error, true
	}
	return nil, false
}
