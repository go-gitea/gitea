// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"
	"unsafe"

	"code.gitea.io/gitea/modules/git/cmd"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// globalCommandArgs global command args for external package setting
	globalCommandArgs []string

	// cmdService represents a command service
	cmdService     cmd.Service
	cmdServiceOnce sync.Once
)

func getCmdService() cmd.Service {
	cmdServiceOnce.Do(func() {
		cmdService = cmd.NewLocalService(setting.Git.Path, setting.RepoRootPath, time.Duration(setting.Git.Timeout.Default)*time.Second)
	})
	return cmdService
}

// CommandProxy represents a command proxy with its subcommands or arguments.
type CommandProxy struct {
	cmd.Command
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
func NewCommand(ctx context.Context, args ...string) *CommandProxy {
	// Make an explicit copy of globalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, len(globalCommandArgs))
	copy(cargs, globalCommandArgs)
	return &CommandProxy{
		Command: getCmdService().NewCommand(ctx, len(cargs), append(cargs, args...)...),
	}
}

// NewCommandNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
func NewCommandNoGlobals(args ...string) *CommandProxy {
	return NewCommandContextNoGlobals(DefaultContext, args...)
}

// NewCommandContextNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
func NewCommandContextNoGlobals(ctx context.Context, args ...string) *CommandProxy {
	return &CommandProxy{
		Command: getCmdService().NewCommand(ctx, 0, args...),
	}
}

// SetParentContext sets the parent context for this command
func (c *CommandProxy) SetParentContext(ctx context.Context) *CommandProxy {
	c.Command.SetParentContext(ctx)
	return c
}

// SetDescription sets the description for this command which be returned on
// c.String()
func (c *CommandProxy) SetDescription(desc string) *CommandProxy {
	c.Command.SetDescription(desc)
	return c
}

// AddArguments adds new argument(s) to the command.
func (c *CommandProxy) AddArguments(args ...string) *CommandProxy {
	c.Command.AddArguments(args...)
	return c
}

type RunStdError interface {
	error
	Stderr() string
}

type runStdError struct {
	err    error
	stderr string
	errMsg string
}

func (r *runStdError) Error() string {
	// the stderr must be in the returned error text, some code only checks `strings.Contains(err.Error(), "git error")`
	if r.errMsg == "" {
		r.errMsg = ConcatenateError(r.err, r.stderr).Error()
	}
	return r.errMsg
}

func (r *runStdError) Unwrap() error {
	return r.err
}

func (r *runStdError) Stderr() string {
	return r.stderr
}

func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b)) // that's what Golang's strings.Builder.String() does (go/src/strings/builder.go)
}

// RunOpts is an alias of cmd.RunOpts
type RunOpts = cmd.RunOpts

// RunStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func (c *CommandProxy) RunStdString(opts *RunOpts) (stdout, stderr string, runErr RunStdError) {
	stdoutBytes, stderrBytes, err := c.RunStdBytes(opts)
	stdout = bytesToString(stdoutBytes)
	stderr = bytesToString(stderrBytes)
	if err != nil {
		return stdout, stderr, &runStdError{err: err, stderr: stderr}
	}
	// even if there is no err, there could still be some stderr output, so we just return stdout/stderr as they are
	return stdout, stderr, nil
}

// RunStdBytes runs the command with options and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func (c *CommandProxy) RunStdBytes(opts *RunOpts) (stdout, stderr []byte, runErr RunStdError) {
	if opts == nil {
		opts = &RunOpts{}
	}
	if opts.Stdout != nil || opts.Stderr != nil {
		// we must panic here, otherwise there would be bugs if developers set Stdin/Stderr by mistake, and it would be very difficult to debug
		panic("stdout and stderr field must be nil when using RunStdBytes")
	}
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	opts.Stdout = stdoutBuf
	opts.Stderr = stderrBuf
	err := c.Run(opts)
	stderr = stderrBuf.Bytes()
	if err != nil {
		return nil, stderr, &runStdError{err: err, stderr: bytesToString(stderr)}
	}
	// even if there is no err, there could still be some stderr output
	return stdoutBuf.Bytes(), stderr, nil
}

// AllowLFSFiltersArgs return globalCommandArgs with lfs filter, it should only be used for tests
func AllowLFSFiltersArgs() []string {
	// Now here we should explicitly allow lfs filters to run
	filteredLFSGlobalArgs := make([]string, len(globalCommandArgs))
	j := 0
	for _, arg := range globalCommandArgs {
		if strings.Contains(arg, "lfs") {
			j--
		} else {
			filteredLFSGlobalArgs[j] = arg
			j++
		}
	}
	return filteredLFSGlobalArgs[:j]
}
