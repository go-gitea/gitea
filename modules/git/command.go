// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git/internal" //nolint:depguard // only this file can use the internal type CmdArg, other files and packages should use AddXxx functions
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

// TrustedCmdArgs returns the trusted arguments for git command.
// It's mainly for passing user-provided and trusted arguments to git command
// In most cases, it shouldn't be used. Use AddXxx function instead
type TrustedCmdArgs []internal.CmdArg

var (
	// globalCommandArgs global command args for external package setting
	globalCommandArgs TrustedCmdArgs

	// defaultCommandExecutionTimeout default command execution timeout duration
	defaultCommandExecutionTimeout = 360 * time.Second
)

// DefaultLocale is the default LC_ALL to run git commands in.
const DefaultLocale = "C"

// Command represents a command with its subcommands or arguments.
type Command struct {
	prog             string
	args             []string
	parentContext    context.Context
	globalArgsLength int
	brokenArgs       []string
}

func logArgSanitize(arg string) string {
	if strings.Contains(arg, "://") && strings.Contains(arg, "@") {
		return util.SanitizeCredentialURLs(arg)
	} else if filepath.IsAbs(arg) {
		base := filepath.Base(arg)
		dir := filepath.Dir(arg)
		return filepath.Join(filepath.Base(dir), base)
	}
	return arg
}

func (c *Command) LogString() string {
	// WARNING: this function is for debugging purposes only. It's much better than old code (which only joins args with space),
	// It's impossible to make a simple and 100% correct implementation of argument quoting for different platforms here.
	debugQuote := func(s string) string {
		if strings.ContainsAny(s, " `'\"\t\r\n") {
			return fmt.Sprintf("%q", s)
		}
		return s
	}
	a := make([]string, 0, len(c.args)+1)
	a = append(a, debugQuote(c.prog))
	if c.globalArgsLength > 0 {
		a = append(a, "...global...")
	}
	for i := c.globalArgsLength; i < len(c.args); i++ {
		a = append(a, debugQuote(logArgSanitize(c.args[i])))
	}
	return strings.Join(a, " ")
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommand(ctx context.Context, args ...internal.CmdArg) *Command {
	// Make an explicit copy of globalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, 0, len(globalCommandArgs)+len(args))
	for _, arg := range globalCommandArgs {
		cargs = append(cargs, string(arg))
	}
	for _, arg := range args {
		cargs = append(cargs, string(arg))
	}
	return &Command{
		prog:             GitExecutable,
		args:             cargs,
		parentContext:    ctx,
		globalArgsLength: len(globalCommandArgs),
	}
}

// NewCommandContextNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommandContextNoGlobals(ctx context.Context, args ...internal.CmdArg) *Command {
	cargs := make([]string, 0, len(args))
	for _, arg := range args {
		cargs = append(cargs, string(arg))
	}
	return &Command{
		prog:          GitExecutable,
		args:          cargs,
		parentContext: ctx,
	}
}

// SetParentContext sets the parent context for this command
func (c *Command) SetParentContext(ctx context.Context) *Command {
	c.parentContext = ctx
	return c
}

// isSafeArgumentValue checks if the argument is safe to be used as a value (not an option)
func isSafeArgumentValue(s string) bool {
	return s == "" || s[0] != '-'
}

// isValidArgumentOption checks if the argument is a valid option (starting with '-').
// It doesn't check whether the option is supported or not
func isValidArgumentOption(s string) bool {
	return s != "" && s[0] == '-'
}

// AddArguments adds new git arguments (option/value) to the command. It only accepts string literals, or trusted CmdArg.
// Type CmdArg is in the internal package, so it can not be used outside of this package directly,
// it makes sure that user-provided arguments won't cause RCE risks.
// User-provided arguments should be passed by other AddXxx functions
func (c *Command) AddArguments(args ...internal.CmdArg) *Command {
	for _, arg := range args {
		c.args = append(c.args, string(arg))
	}
	return c
}

// AddOptionValues adds a new option with a list of non-option values
// For example: AddOptionValues("--opt", val) means 2 arguments: {"--opt", val}.
// The values are treated as dynamic argument values. It equals to: AddArguments("--opt") then AddDynamicArguments(val).
func (c *Command) AddOptionValues(opt internal.CmdArg, args ...string) *Command {
	if !isValidArgumentOption(string(opt)) {
		c.brokenArgs = append(c.brokenArgs, string(opt))
		return c
	}
	c.args = append(c.args, string(opt))
	c.AddDynamicArguments(args...)
	return c
}

// AddOptionFormat adds a new option with a format string and arguments
// For example: AddOptionFormat("--opt=%s %s", val1, val2) means 1 argument: {"--opt=val1 val2"}.
func (c *Command) AddOptionFormat(opt string, args ...any) *Command {
	if !isValidArgumentOption(opt) {
		c.brokenArgs = append(c.brokenArgs, opt)
		return c
	}
	// a quick check to make sure the format string matches the number of arguments, to find low-level mistakes ASAP
	if strings.Count(strings.ReplaceAll(opt, "%%", ""), "%") != len(args) {
		c.brokenArgs = append(c.brokenArgs, opt)
		return c
	}
	s := fmt.Sprintf(opt, args...)
	c.args = append(c.args, s)
	return c
}

// AddDynamicArguments adds new dynamic argument values to the command.
// The arguments may come from user input and can not be trusted, so no leading '-' is allowed to avoid passing options.
// TODO: in the future, this function can be renamed to AddArgumentValues
func (c *Command) AddDynamicArguments(args ...string) *Command {
	for _, arg := range args {
		if !isSafeArgumentValue(arg) {
			c.brokenArgs = append(c.brokenArgs, arg)
		}
	}
	if len(c.brokenArgs) != 0 {
		return c
	}
	c.args = append(c.args, args...)
	return c
}

// AddDashesAndList adds the "--" and then add the list as arguments, it's usually for adding file list
// At the moment, this function can be only called once, maybe in future it can be refactored to support multiple calls (if necessary)
func (c *Command) AddDashesAndList(list ...string) *Command {
	c.args = append(c.args, "--")
	// Some old code also checks `arg != ""`, IMO it's not necessary.
	// If the check is needed, the list should be prepared before the call to this function
	c.args = append(c.args, list...)
	return c
}

// ToTrustedCmdArgs converts a list of strings (trusted as argument) to TrustedCmdArgs
// In most cases, it shouldn't be used. Use NewCommand().AddXxx() function instead
func ToTrustedCmdArgs(args []string) TrustedCmdArgs {
	ret := make(TrustedCmdArgs, len(args))
	for i, arg := range args {
		ret[i] = internal.CmdArg(arg)
	}
	return ret
}

// RunOpts represents parameters to run the command. If UseContextTimeout is specified, then Timeout is ignored.
type RunOpts struct {
	Env               []string
	Timeout           time.Duration
	UseContextTimeout bool

	// Dir is the working dir for the git command, however:
	// FIXME: this could be incorrect in many cases, for example:
	// * /some/path/.git
	// * /some/path/.git/gitea-data/data/repositories/user/repo.git
	// If "user/repo.git" is invalid/broken, then running git command in it will use "/some/path/.git", and produce unexpected results
	// The correct approach is to use `--git-dir" global argument
	Dir string

	Stdout, Stderr io.Writer

	// Stdin is used for passing input to the command
	// The caller must make sure the Stdin writer is closed properly to finish the Run function.
	// Otherwise, the Run function may hang for long time or forever, especially when the Git's context deadline is not the same as the caller's.
	// Some common mistakes:
	// * `defer stdinWriter.Close()` then call `cmd.Run()`: the Run() would never return if the command is killed by timeout
	// * `go { case <- parentContext.Done(): stdinWriter.Close() }` with `cmd.Run(DefaultTimeout)`: the command would have been killed by timeout but the Run doesn't return until stdinWriter.Close()
	// * `go { if stdoutReader.Read() err != nil: stdinWriter.Close() }` with `cmd.Run()`: the stdoutReader may never return error if the command is killed by timeout
	// In the future, ideally the git module itself should have full control of the stdin, to avoid such problems and make it easier to refactor to a better architecture.
	Stdin io.Reader

	PipelineFunc func(context.Context, context.CancelFunc) error
}

func commonBaseEnvs() []string {
	envs := []string{
		// Make Gitea use internal git config only, to prevent conflicts with user's git config
		// It's better to use GIT_CONFIG_GLOBAL, but it requires git >= 2.32, so we still use HOME at the moment.
		"HOME=" + HomeDir(),
		// Avoid using system git config, it would cause problems (eg: use macOS osxkeychain to show a modal dialog, auto installing lfs hooks)
		// This might be a breaking change in 1.24, because some users said that they have put some configs like "receive.certNonceSeed" in "/etc/gitconfig"
		// For these users, they need to migrate the necessary configs to Gitea's git config file manually.
		"GIT_CONFIG_NOSYSTEM=1",
		// Ignore replace references (https://git-scm.com/docs/git-replace)
		"GIT_NO_REPLACE_OBJECTS=1",
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

// CommonCmdServEnvs is like CommonGitCmdEnvs, but it only returns minimal required environment variables for the "gitea serv" command
func CommonCmdServEnvs() []string {
	return commonBaseEnvs()
}

var ErrBrokenCommand = errors.New("git command is broken")

// Run runs the command with the RunOpts
func (c *Command) Run(opts *RunOpts) error {
	return c.run(1, opts)
}

func (c *Command) run(skip int, opts *RunOpts) error {
	if len(c.brokenArgs) != 0 {
		log.Error("git command is broken: %s, broken args: %s", c.LogString(), strings.Join(c.brokenArgs, " "))
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

	var desc string
	callerInfo := util.CallerFuncName(1 /* util */ + 1 /* this */ + skip /* parent */)
	if pos := strings.LastIndex(callerInfo, "/"); pos >= 0 {
		callerInfo = callerInfo[pos+1:]
	}
	// these logs are for debugging purposes only, so no guarantee of correctness or stability
	desc = fmt.Sprintf("git.Run(by:%s, repo:%s): %s", callerInfo, logArgSanitize(opts.Dir), c.LogString())
	log.Debug("git.Command: %s", desc)

	var ctx context.Context
	var cancel context.CancelFunc
	var finished context.CancelFunc

	if opts.UseContextTimeout {
		ctx, cancel, finished = process.GetManager().AddContext(c.parentContext, desc)
	} else {
		ctx, cancel, finished = process.GetManager().AddContextTimeout(c.parentContext, timeout, desc)
	}
	defer finished()

	startTime := time.Now()

	cmd := exec.CommandContext(ctx, c.prog, c.args...)
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

	err := cmd.Wait()
	elapsed := time.Since(startTime)
	if elapsed > time.Second {
		log.Debug("slow git.Command.Run: %s (%s)", c, elapsed)
	}

	// We need to check if the context is canceled by the program on Windows.
	// This is because Windows does not have signal checking when terminating the process.
	// It always returns exit code 1, unlike Linux, which has many exit codes for signals.
	if runtime.GOOS == "windows" &&
		err != nil &&
		err.Error() == "" &&
		cmd.ProcessState.ExitCode() == 1 &&
		ctx.Err() == context.Canceled {
		return ctx.Err()
	}

	if err != nil && ctx.Err() != context.DeadlineExceeded {
		return err
	}

	return ctx.Err()
}

type RunStdError interface {
	error
	Unwrap() error
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

func IsErrorExitCode(err error, code int) bool {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode() == code
	}
	return false
}

// RunStdString runs the command with options and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func (c *Command) RunStdString(opts *RunOpts) (stdout, stderr string, runErr RunStdError) {
	stdoutBytes, stderrBytes, err := c.runStdBytes(opts)
	stdout = util.UnsafeBytesToString(stdoutBytes)
	stderr = util.UnsafeBytesToString(stderrBytes)
	if err != nil {
		return stdout, stderr, &runStdError{err: err, stderr: stderr}
	}
	// even if there is no err, there could still be some stderr output, so we just return stdout/stderr as they are
	return stdout, stderr, nil
}

// RunStdBytes runs the command with options and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func (c *Command) RunStdBytes(opts *RunOpts) (stdout, stderr []byte, runErr RunStdError) {
	return c.runStdBytes(opts)
}

func (c *Command) runStdBytes(opts *RunOpts) (stdout, stderr []byte, runErr RunStdError) {
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

	err := c.run(2, newOpts)
	stderr = stderrBuf.Bytes()
	if err != nil {
		return nil, stderr, &runStdError{err: err, stderr: util.UnsafeBytesToString(stderr)}
	}
	// even if there is no err, there could still be some stderr output
	return stdoutBuf.Bytes(), stderr, nil
}

// AllowLFSFiltersArgs return globalCommandArgs with lfs filter, it should only be used for tests
func AllowLFSFiltersArgs() TrustedCmdArgs {
	// Now here we should explicitly allow lfs filters to run
	filteredLFSGlobalArgs := make(TrustedCmdArgs, len(globalCommandArgs))
	j := 0
	for _, arg := range globalCommandArgs {
		if strings.Contains(string(arg), "lfs") {
			j--
		} else {
			filteredLFSGlobalArgs[j] = arg
			j++
		}
	}
	return filteredLFSGlobalArgs[:j]
}
