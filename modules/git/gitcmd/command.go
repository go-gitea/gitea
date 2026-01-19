// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git/internal" //nolint:depguard // only this file can use the internal type CmdArg, other files and packages should use AddXxx functions
	"code.gitea.io/gitea/modules/gtprof"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

// TrustedCmdArgs returns the trusted arguments for git command.
// It's mainly for passing user-provided and trusted arguments to git command
// In most cases, it shouldn't be used. Use AddXxx function instead
type TrustedCmdArgs []internal.CmdArg

// DefaultLocale is the default LC_ALL to run git commands in.
const DefaultLocale = "C"

// Command represents a command with its subcommands or arguments.
type Command struct {
	callerInfo string
	prog       string
	args       []string
	preErrors  []error
	configArgs []string
	opts       runOpts

	cmd *exec.Cmd

	cmdCtx       context.Context
	cmdCancel    process.CancelCauseFunc
	cmdFinished  process.FinishedFunc
	cmdStartTime time.Time

	cmdStdinWriter   *io.WriteCloser
	cmdStdoutReader  *io.ReadCloser
	cmdStderrReader  *io.ReadCloser
	cmdManagedStderr *bytes.Buffer
}

func logArgSanitize(arg string) string {
	if strings.Contains(arg, "://") && strings.Contains(arg, "@") {
		return util.SanitizeCredentialURLs(arg)
	} else if filepath.IsAbs(arg) {
		base := filepath.Base(arg)
		dir := filepath.Dir(arg)
		return ".../" + filepath.Join(filepath.Base(dir), base)
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
	for i := 0; i < len(c.args); i++ {
		a = append(a, debugQuote(logArgSanitize(c.args[i])))
	}
	return strings.Join(a, " ")
}

func (c *Command) ProcessState() string {
	if c.cmd == nil {
		return ""
	}
	return c.cmd.ProcessState.String()
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
func NewCommand(args ...internal.CmdArg) *Command {
	cargs := make([]string, 0, len(args))
	for _, arg := range args {
		cargs = append(cargs, string(arg))
	}
	return &Command{
		prog: GitExecutable,
		args: cargs,
	}
}

func (c *Command) handlePreErrorBrokenCommand(arg string) {
	c.preErrors = append(c.preErrors, util.ErrorWrap(ErrBrokenCommand, `broken git command argument %q`, arg))
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
		c.handlePreErrorBrokenCommand(string(opt))
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
		c.handlePreErrorBrokenCommand(opt)
		return c
	}
	// a quick check to make sure the format string matches the number of arguments, to find low-level mistakes ASAP
	if strings.Count(strings.ReplaceAll(opt, "%%", ""), "%") != len(args) {
		c.handlePreErrorBrokenCommand(opt)
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
			c.handlePreErrorBrokenCommand(arg)
		}
	}
	if len(c.preErrors) != 0 {
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

func (c *Command) AddConfig(key, value string) *Command {
	kv := key + "=" + value
	if !isSafeArgumentValue(kv) {
		c.handlePreErrorBrokenCommand(kv)
	} else {
		c.configArgs = append(c.configArgs, "-c", kv)
	}
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

type runOpts struct {
	Env     []string
	Timeout time.Duration

	// Dir is the working dir for the git command, however:
	// FIXME: this could be incorrect in many cases, for example:
	// * /some/path/.git
	// * /some/path/.git/gitea-data/data/repositories/user/repo.git
	// If "user/repo.git" is invalid/broken, then running git command in it will use "/some/path/.git", and produce unexpected results
	// The correct approach is to use `--git-dir" global argument
	Dir string

	Stdout io.Writer

	// Stdin is used for passing input to the command
	// The caller must make sure the Stdin writer is closed properly to finish the Run function.
	// Otherwise, the Run function may hang for long time or forever, especially when the Git's context deadline is not the same as the caller's.
	// Some common mistakes:
	// * `defer stdinWriter.Close()` then call `cmd.Run()`: the Run() would never return if the command is killed by timeout
	// * `go { case <- parentContext.Done(): stdinWriter.Close() }` with `cmd.Run(DefaultTimeout)`: the command would have been killed by timeout but the Run doesn't return until stdinWriter.Close()
	// * `go { if stdoutReader.Read() err != nil: stdinWriter.Close() }` with `cmd.Run()`: the stdoutReader may never return error if the command is killed by timeout
	// In the future, ideally the git module itself should have full control of the stdin, to avoid such problems and make it easier to refactor to a better architecture.
	// Use new functions like WithStdinWriter to avoid such problems.
	Stdin io.Reader

	PipelineFunc func(Context) error
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

func (c *Command) WithDir(dir string) *Command {
	c.opts.Dir = dir
	return c
}

func (c *Command) WithEnv(env []string) *Command {
	c.opts.Env = env
	return c
}

func (c *Command) WithTimeout(timeout time.Duration) *Command {
	c.opts.Timeout = timeout
	return c
}

func (c *Command) WithStdoutReader(r *io.ReadCloser) *Command {
	c.cmdStdoutReader = r
	return c
}

// WithStdout is deprecated, use WithStdoutReader instead
func (c *Command) WithStdout(stdout io.Writer) *Command {
	c.opts.Stdout = stdout
	return c
}

func (c *Command) WithStderrReader(r *io.ReadCloser) *Command {
	c.cmdStderrReader = r
	return c
}

func (c *Command) WithStdinWriter(w *io.WriteCloser) *Command {
	c.cmdStdinWriter = w
	return c
}

// WithStdin is deprecated, use WithStdinWriter instead
func (c *Command) WithStdin(stdin io.Reader) *Command {
	c.opts.Stdin = stdin
	return c
}

func (c *Command) WithPipelineFunc(f func(Context) error) *Command {
	c.opts.PipelineFunc = f
	return c
}

// WithParentCallerInfo can be used to set the caller info (usually function name) of the parent function of the caller.
// For most cases, "Run" family functions can get its caller info automatically
// But if you need to call "Run" family functions in a wrapper function: "FeatureFunc -> GeneralWrapperFunc -> RunXxx",
// then you can to call this function in GeneralWrapperFunc to set the caller info of FeatureFunc.
// The caller info can only be set once.
func (c *Command) WithParentCallerInfo(optInfo ...string) *Command {
	if c.callerInfo != "" {
		return c
	}
	if len(optInfo) > 0 {
		c.callerInfo = optInfo[0]
		return c
	}
	skip := 1 /*parent "wrap/run" functions*/ + 1 /*this function*/
	callerFuncName := util.CallerFuncName(skip)
	callerInfo := callerFuncName
	if pos := strings.LastIndex(callerInfo, "/"); pos >= 0 {
		callerInfo = callerInfo[pos+1:]
	}
	c.callerInfo = callerInfo
	return c
}

func (c *Command) Start(ctx context.Context) (retErr error) {
	if c.cmd != nil {
		// this is a programming error, it will cause serious deadlock problems, so it must be fixed.
		panic("git command has already been started")
	}

	defer func() {
		if retErr != nil {
			// release the pipes to avoid resource leak
			c.closeStdioPipes()
			// if error occurs, we must also finish the task, otherwise, cmdFinished will be called in "Wait" function
			if c.cmdFinished != nil {
				c.cmdFinished()
			}
		}
	}()

	if len(c.preErrors) != 0 {
		// In most cases, such error shouldn't happen. If it happens, it must be a programming error, so we log it as error level with more details
		err := errors.Join(c.preErrors...)
		log.Error("git command: %s, error: %s", c.LogString(), err)
		return err
	}

	cmdLogString := c.LogString()
	if c.callerInfo == "" {
		c.WithParentCallerInfo()
	}
	// these logs are for debugging purposes only, so no guarantee of correctness or stability
	desc := fmt.Sprintf("git.Run(by:%s, repo:%s): %s", c.callerInfo, logArgSanitize(c.opts.Dir), cmdLogString)
	log.Debug("git.Command: %s", desc)

	_, span := gtprof.GetTracer().Start(ctx, gtprof.TraceSpanGitRun)
	defer span.End()
	span.SetAttributeString(gtprof.TraceAttrFuncCaller, c.callerInfo)
	span.SetAttributeString(gtprof.TraceAttrGitCommand, cmdLogString)

	if c.opts.Timeout <= 0 {
		c.cmdCtx, c.cmdCancel, c.cmdFinished = process.GetManager().AddContext(ctx, desc)
	} else {
		c.cmdCtx, c.cmdCancel, c.cmdFinished = process.GetManager().AddContextTimeout(ctx, c.opts.Timeout, desc)
	}

	c.cmdStartTime = time.Now()

	c.cmd = exec.CommandContext(ctx, c.prog, append(c.configArgs, c.args...)...)
	if c.opts.Env == nil {
		c.cmd.Env = os.Environ()
	} else {
		c.cmd.Env = c.opts.Env
	}

	process.SetSysProcAttribute(c.cmd)
	c.cmd.Env = append(c.cmd.Env, CommonGitCmdEnvs()...)
	c.cmd.Dir = c.opts.Dir
	c.cmd.Stdout = c.opts.Stdout
	c.cmd.Stdin = c.opts.Stdin

	if _, err := safeAssignPipe(c.cmdStdinWriter, c.cmd.StdinPipe); err != nil {
		return err
	}
	if _, err := safeAssignPipe(c.cmdStdoutReader, c.cmd.StdoutPipe); err != nil {
		return err
	}
	if _, err := safeAssignPipe(c.cmdStderrReader, c.cmd.StderrPipe); err != nil {
		return err
	}

	if c.cmdManagedStderr != nil {
		if c.cmd.Stderr != nil {
			panic("CombineStderr needs managed (but not caller-provided) stderr pipe")
		}
		c.cmd.Stderr = c.cmdManagedStderr
	}
	return c.cmd.Start()
}

func (c *Command) closeStdioPipes() {
	safeClosePtrCloser(c.cmdStdoutReader)
	safeClosePtrCloser(c.cmdStderrReader)
	safeClosePtrCloser(c.cmdStdinWriter)
}

func (c *Command) Wait() error {
	defer func() {
		c.closeStdioPipes()
		c.cmdFinished()
	}()

	if c.opts.PipelineFunc != nil {
		errCallback := c.opts.PipelineFunc(&cmdContext{Context: c.cmdCtx, cmd: c})
		// after the pipeline function returns, we can safely cancel the command context and close the stdio pipes
		c.cmdCancel(errCallback)
		c.closeStdioPipes()
		errWait := c.cmd.Wait()
		errCause := context.Cause(c.cmdCtx)
		// the pipeline function should be able to know whether it succeeds or fails
		if errCallback == nil && (errCause == nil || errors.Is(errCause, context.Canceled)) {
			return nil
		}
		return errors.Join(errCallback, errCause, errWait)
	}

	// there might be other goroutines using the context or pipes, so we just wait for the command to finish
	errWait := c.cmd.Wait()
	elapsed := time.Since(c.cmdStartTime)
	if elapsed > time.Second {
		log.Debug("slow git.Command.Run: %s (%s)", c, elapsed) // TODO: no need to log this for long-running commands
	}

	// Here the logic is different from "PipelineFunc" case,
	// because PipelineFunc can return error if it fails, it knows whether it succeeds or fails.
	// But in normal case, the caller just runs the git command, the command's exit code is the source of truth.
	// If the caller need to know whether the command error is caused by cancellation, it should check the "err" by itself.
	errCause := context.Cause(c.cmdCtx)
	return errors.Join(errCause, errWait)
}

func (c *Command) StartWithStderr(ctx context.Context) RunStdError {
	c.cmdManagedStderr = &bytes.Buffer{}
	err := c.Start(ctx)
	if err != nil {
		return &runStdError{err: err}
	}
	return nil
}

func (c *Command) WaitWithStderr() RunStdError {
	if c.cmdManagedStderr == nil {
		panic("CombineStderr needs managed (but not caller-provided) stderr pipe")
	}
	errWait := c.Wait()
	if errWait == nil {
		// if no exec error but only stderr output, the stderr output is still saved in "c.cmdManagedStderr" and can be read later
		return nil
	}
	return &runStdError{err: errWait, stderr: util.UnsafeBytesToString(c.cmdManagedStderr.Bytes())}
}

func (c *Command) RunWithStderr(ctx context.Context) RunStdError {
	if err := c.StartWithStderr(ctx); err != nil {
		return &runStdError{err: err}
	}
	return c.WaitWithStderr()
}

func (c *Command) Run(ctx context.Context) (err error) {
	if err = c.Start(ctx); err != nil {
		return err
	}
	return c.Wait()
}

// RunStdString runs the command and returns stdout/stderr as string. and store stderr to returned error (err combined with stderr).
func (c *Command) RunStdString(ctx context.Context) (stdout, stderr string, runErr RunStdError) {
	stdoutBytes, stderrBytes, runErr := c.WithParentCallerInfo().runStdBytes(ctx)
	return util.UnsafeBytesToString(stdoutBytes), util.UnsafeBytesToString(stderrBytes), runErr
}

// RunStdBytes runs the command and returns stdout/stderr as bytes. and store stderr to returned error (err combined with stderr).
func (c *Command) RunStdBytes(ctx context.Context) (stdout, stderr []byte, runErr RunStdError) {
	return c.WithParentCallerInfo().runStdBytes(ctx)
}

func (c *Command) runStdBytes(ctx context.Context) ([]byte, []byte, RunStdError) {
	if c.opts.Stdout != nil || c.cmdStdoutReader != nil || c.cmdStderrReader != nil {
		// we must panic here, otherwise there would be bugs if developers set Stdin/Stderr by mistake, and it would be very difficult to debug
		panic("stdout and stderr field must be nil when using RunStdBytes")
	}
	stdoutBuf := &bytes.Buffer{}
	err := c.WithParentCallerInfo().
		WithStdout(stdoutBuf).
		RunWithStderr(ctx)
	return stdoutBuf.Bytes(), c.cmdManagedStderr.Bytes(), err
}

func (c *Command) DebugKill() {
	_ = c.cmd.Process.Kill()
}
