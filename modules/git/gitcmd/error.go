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
	LogString() string
}

type runStdError struct {
	err    error  // usually the low-level error like `*exec.ExitError`
	stderr string // git command's stderr output
}

// Error deliberately omits the stderr, which can echo remote-controlled text into
// responses and stored messages. Use Stderr or IsStderr to inspect it.
func (r *runStdError) Error() string {
	return r.err.Error()
}

// LogString keeps the stderr in logs, where the detail is wanted
func (r *runStdError) LogString() string {
	return fmt.Sprintf("%s - %s", r.err.Error(), strings.TrimSpace(r.stderr))
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
	StderrContains string
	StderrWildcard string
)

const (
	StderrNotValidObjectName StderrPrefix = "fatal: not a valid object name"
	StderrNotTreeObject      StderrPrefix = "fatal: not a tree object"
	StderrPathSpec           StderrPrefix = "fatal: pathspec"
	StderrBadRevision        StderrPrefix = "fatal: bad revision"

	StderrNoSuchRemote1 StderrPrefix = "fatal: no such remote" // git < 2.30, exit status 128
	StderrNoSuchRemote2 StderrPrefix = "error: no such remote" // git >= 2.30. exit status 2

	// these are not at the start of stderr, git prints the remote and progress lines first
	StderrAuthenticationFailed StderrContains = "Authentication failed"
	StderrCouldNotReadUsername StderrContains = "could not read Username"
	StderrNeededSingleRevision StderrContains = "Needed a single revision"
	StderrNotAValidRef         StderrContains = "not a valid ref"
	StderrNotAValidTagName     StderrContains = "is not a valid tag name"
	StderrRefAlreadyExists     StderrContains = "already exists"

	StderrUnknownRevisionOrPath StderrWildcard = "fatal: *: unknown revision or path not in the working tree"
	StderrNoMergeBase           StderrWildcard = "fatal: *: no merge base"
	StderrTagNotFound           StderrWildcard = "error: tag *not found"
)

func IsStderr[T StderrPrefix | StderrContains | StderrWildcard](err error, check T) bool {
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
	case StderrContains:
		return strings.Contains(stderr, string(check))
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
