// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

var (
	// globalCommandArgs global command args for external package setting
	globalCommandArgs []string

	// defaultCommandExecutionTimeout default command execution timeout duration
	defaultCommandExecutionTimeout = 360 * time.Second
)

// DefaultLocale is the default LC_ALL to run git commands in.
const DefaultLocale = "C"

// Command represents a command with its subcommands or arguments.
type Command struct {
	name             string
	args             []string
	parentContext    context.Context
	desc             string
	globalArgsLength int
	brokenArgs       []string
}

func (c *Command) String() string {
	if len(c.args) == 0 {
		return c.name
	}
	return fmt.Sprintf("%s %s", c.name, strings.Join(c.args, " "))
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommand(ctx context.Context, args ...string) *Command {
	// Make an explicit copy of globalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, len(globalCommandArgs))
	copy(cargs, globalCommandArgs)
	return &Command{
		name:             GitExecutable,
		args:             append(cargs, args...),
		parentContext:    ctx,
		globalArgsLength: len(globalCommandArgs),
	}
}

// NewCommandNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommandNoGlobals(args ...string) *Command {
	return NewCommandContextNoGlobals(DefaultContext, args...)
}

// NewCommandContextNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommandContextNoGlobals(ctx context.Context, args ...string) *Command {
	return &Command{
		name:          GitExecutable,
		args:          args,
		parentContext: ctx,
	}
}

// SetParentContext sets the parent context for this command
func (c *Command) SetParentContext(ctx context.Context) *Command {
	c.parentContext = ctx
	return c
}

// SetDescription sets the description for this command which be returned on
// c.String()
func (c *Command) SetDescription(desc string) *Command {
	c.desc = desc
	return c
}

// AddArguments adds new argument(s) to the command. Each argument must be safe to be trusted.
// User-provided arguments should be passed to AddDynamicArguments instead.
func (c *Command) AddArguments(args ...string) *Command {
	c.args = append(c.args, args...)
	return c
}

// AddDynamicArguments adds new dynamic argument(s) to the command.
// The arguments may come from user input and can not be trusted, so no leading '-' is allowed to avoid passing options
func (c *Command) AddDynamicArguments(args ...string) *Command {
	for _, arg := range args {
		if arg != "" && arg[0] == '-' {
			c.brokenArgs = append(c.brokenArgs, arg)
		}
	}
	if len(c.brokenArgs) != 0 {
		return c
	}
	c.args = append(c.args, args...)
	return c
}

// RunOpts represents parameters to run the command. If UseContextTimeout is specified, then Timeout is ignored.
type RunOpts struct {
	Env               []string
	Timeout           time.Duration
	UseContextTimeout bool
	Dir               string
	Stdout, Stderr    io.Writer
	Stdin             io.Reader
	PipelineFunc      func(context.Context, context.CancelFunc) error
}

func commonBaseEnvs() []string {
	// at the moment, do not set "GIT_CONFIG_NOSYSTEM", users may have put some configs like "receive.certNonceSeed" in it
	envs := []string{
		"HOME=" + HomeDir(),        // make Gitea use internal git config only, to prevent conflicts with user's git config
		"GIT_NO_REPLACE_OBJECTS=1", // ignore replace references (https://git-scm.com/docs/git-replace)
	}

	// some environment variables should be passed to git command
	passThroughEnvKeys := []string{
		"GNUPGHOME", // git may call gnupg to do commit signing
	}
	for _, key := range passThroughEnvKeys {
		if val, ok := os.LookupEnv(key); ok {
			envs = append(envs, key+"="+val)
		}
	}
	return envs
}

// CommonGitCmdEnvs returns the common environment variables for a "git" command.
func CommonGitCmdEnvs() []string {
	return append(commonBaseEnvs(), []string{
		"LC_ALL=" + DefaultLocale,
		"GIT_TERMINAL_PROMPT=0", // avoid prompting for credentials interactively, supported since git v2.3
	}...)
}

// CommonCmdServEnvs is like CommonGitCmdEnvs but it only returns minimal required environment variables for the "gitea serv" command
func CommonCmdServEnvs() []string {
	return commonBaseEnvs()
}

var ErrBrokenCommand = errors.New("git command is broken")

// Run runs the command with the RunOpts
func (c *Command) Run(opts *RunOpts) error {
	if len(c.brokenArgs) != 0 {
		log.Error("git command is broken: %s, broken args: %s", c.String(), strings.Join(c.brokenArgs, " "))
		return ErrBrokenCommand
	}
	if opts == nil {
		opts = &RunOpts{}
	}

	// We must not change the provided options
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultCommandExecutionTimeout
	}

	if len(opts.Dir) == 0 {
		log.Debug("%s", c)
	} else {
		log.Debug("%s: %v", opts.Dir, c)
	}

	desc := c.desc
	if desc == "" {
		args := c.args[c.globalArgsLength:]
		var argSensitiveURLIndexes []int
		for i, arg := range c.args {
			if strings.Contains(arg, "://") && strings.Contains(arg, "@") {
				argSensitiveURLIndexes = append(argSensitiveURLIndexes, i)
			}
		}
		if len(argSensitiveURLIndexes) > 0 {
			args = make([]string, len(c.args))
			copy(args, c.args)
			for _, urlArgIndex := range argSensitiveURLIndexes {
				args[urlArgIndex] = util.SanitizeCredentialURLs(args[urlArgIndex])
			}
		}
		desc = fmt.Sprintf("%s %s [repo_path: %s]", c.name, strings.Join(args, " "), opts.Dir)
	}

	var ctx context.Context
	var cancel context.CancelFunc
	var finished context.CancelFunc

	if opts.UseContextTimeout {
		ctx, cancel, finished = process.GetManager().AddContext(c.parentContext, desc)
	} else {
		ctx, cancel, finished = process.GetManager().AddContextTimeout(c.parentContext, timeout, desc)
	}
	defer finished()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if opts.Env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = opts.Env
	}

	process.SetSysProcAttribute(cmd)
	cmd.Env = append(cmd.Env, CommonGitCmdEnvs()...)
	cmd.Dir = opts.Dir
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	cmd.Stdin = opts.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	if opts.PipelineFunc != nil {
		err := opts.PipelineFunc(ctx, cancel)
		if err != nil {
			cancel()
			_ = cmd.Wait()
			return err
		}
	}

	if err := cmd.Wait(); err != nil && ctx.Err() != context.DeadlineExceeded {
		return err
	}

	return ctx.Err()
}

type RunStdError interface {
	error
	Unwrap() error
	Stderr() string
	IsExitCode(code int) bool
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

func (r *runStdError) IsExitCode(code int) bool {
	var exitError *exec.ExitError
	if errors.As(r.err, &exitError) {
		return exitError.ExitCode() == code
	}
	return false
}

func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b)) // that's what Golang's strings.Builder.String() does (go/src/strings/builder.go)
}

// RunStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func (c *Command) RunStdString(opts *RunOpts) (stdout, stderr string, runErr RunStdError) {
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
func (c *Command) RunStdBytes(opts *RunOpts) (stdout, stderr []byte, runErr RunStdError) {
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
		Env:               opts.Env,
		Timeout:           opts.Timeout,
		UseContextTimeout: opts.UseContextTimeout,
		Dir:               opts.Dir,
		Stdout:            stdoutBuf,
		Stderr:            stderrBuf,
		Stdin:             opts.Stdin,
		PipelineFunc:      opts.PipelineFunc,
	}

	err := c.Run(newOpts)
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
