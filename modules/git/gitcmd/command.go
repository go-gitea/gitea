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

	parentPipeFiles   []*os.File
	parentPipeReaders []*os.File
	childrenPipeFiles []*os.File

	// only os.Pipe and in-memory buffers can work with Stdin safely, see https://github.com/golang/go/issues/77227 if the command would exit unexpectedly
	cmdStdin  io.Reader
	cmdStdout io.Writer
	cmdStderr io.Writer

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
		"LC_ALL=C",              // ensure git output is in English, error messages are parsed in English
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

func (c *Command) makeStdoutStderr(w *io.Writer) (PipeReader, func()) {
	pr, pw, err := os.Pipe()
	if err != nil {
		c.preErrors = append(c.preErrors, err)
		return &pipeNull{err}, func() {}
	}
	c.childrenPipeFiles = append(c.childrenPipeFiles, pw)
	c.parentPipeFiles = append(c.parentPipeFiles, pr)
	c.parentPipeReaders = append(c.parentPipeReaders, pr)
	*w /* stdout, stderr */ = pw
	return &pipeReader{f: pr}, func() { pr.Close() }
}

// MakeStdinPipe creates a writer for the command's stdin.
// The returned closer function must be called by the caller to close the pipe.
func (c *Command) MakeStdinPipe() (writer PipeWriter, closer func()) {
	pr, pw, err := os.Pipe()
	if err != nil {
		c.preErrors = append(c.preErrors, err)
		return &pipeNull{err}, func() {}
	}
	c.childrenPipeFiles = append(c.childrenPipeFiles, pr)
	c.parentPipeFiles = append(c.parentPipeFiles, pw)
	c.cmdStdin = pr
	return &pipeWriter{pw}, func() { pw.Close() }
}

// MakeStdoutPipe creates a reader for the command's stdout.
// The returned closer function must be called by the caller to close the pipe.
// After the pipe reader is closed, the unread data will be discarded.
//
// If the process (git command) still tries to write after the pipe is closed, the Wait error will be "signal: broken pipe".
// WithPipelineFunc + Run won't return "broken pipe" error in this case if the callback returns no error.
// But if you are calling Start / Wait family functions, you should either drain the pipe before close it, or handle the Wait error correctly.
func (c *Command) MakeStdoutPipe() (reader PipeReader, closer func()) {
	return c.makeStdoutStderr(&c.cmdStdout)
}

// MakeStderrPipe is like MakeStdoutPipe, but for stderr.
func (c *Command) MakeStderrPipe() (reader PipeReader, closer func()) {
	return c.makeStdoutStderr(&c.cmdStderr)
}

func (c *Command) MakeStdinStdoutPipe() (stdin PipeWriter, stdout PipeReader, closer func()) {
	stdin, stdinClose := c.MakeStdinPipe()
	stdout, stdoutClose := c.MakeStdoutPipe()
	return stdin, stdout, func() {
		stdinClose()
		stdoutClose()
	}
}

func (c *Command) WithStdinBytes(stdin []byte) *Command {
	c.cmdStdin = bytes.NewReader(stdin)
	return c
}

func (c *Command) WithStdoutBuffer(w PipeBufferWriter) *Command {
	c.cmdStdout = w
	return c
}

// WithStdinCopy and WithStdoutCopy are general functions that accept any io.Reader / io.Writer.
// In this case, Golang exec.Cmd will start new internal goroutines to do io.Copy between pipes and provided Reader/Writer.
// If the reader or writer is blocked and never returns, then the io.Copy won't finish, then exec.Cmd.Wait won't return, which may cause deadlocks.
// A typical deadlock example is:
// * `r,w:=io.Pipe(); cmd.Stdin=r; defer w.Close(); cmd.Run()`: the Run() will never return because stdin reader is blocked forever and w.Close() will never be called.
// If the reader/writer won't block forever (for example: read from a file or buffer), then these functions are safe to use.
func (c *Command) WithStdinCopy(w io.Reader) *Command {
	c.cmdStdin = w
	return c
}

func (c *Command) WithStdoutCopy(w io.Writer) *Command {
	c.cmdStdout = w
	return c
}

// WithPipelineFunc sets the pipeline function for the command.
// The pipeline function will be called in the Run / Wait function after the command is started successfully.
// The function can read/write from/to the command's stdio pipes (if any).
// The pipeline function can cancel (kill) the command by calling ctx.CancelPipeline before the command finishes.
// The returned error of Run / Wait can be joined errors from the pipeline function, context cause, and command exit error.
// Caller can get the pipeline function's error (if any) by UnwrapPipelineError.
func (c *Command) WithPipelineFunc(f func(ctx Context) error) *Command {
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
		c.closePipeFiles(c.childrenPipeFiles)
		if retErr != nil {
			// release the pipes to avoid resource leak since the command failed to start
			c.closePipeFiles(c.parentPipeFiles)
			// if error occurs, we must also finish the task, otherwise, cmdFinished will be called in "Wait" function
			if c.cmdFinished != nil {
				c.cmdFinished()
			}
		}
	}()

	if len(c.preErrors) != 0 {
		// In most cases, such error shouldn't happen. If it happens, log it as error level with more details
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

	c.cmd = exec.CommandContext(c.cmdCtx, c.prog, append(c.configArgs, c.args...)...)
	if c.opts.Env == nil {
		c.cmd.Env = os.Environ()
	} else {
		c.cmd.Env = c.opts.Env
	}

	process.SetSysProcAttribute(c.cmd)
	c.cmd.Env = append(c.cmd.Env, CommonGitCmdEnvs()...)
	c.cmd.Dir = c.opts.Dir
	c.cmd.Stdout = c.cmdStdout
	c.cmd.Stdin = c.cmdStdin
	c.cmd.Stderr = c.cmdStderr
	return c.cmd.Start()
}

func (c *Command) closePipeFiles(files []*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}

func (c *Command) discardPipeReaders(files []*os.File) {
	for _, f := range files {
		_, _ = io.Copy(io.Discard, f)
	}
}

func (c *Command) Wait() error {
	defer func() {
		// The reader in another goroutine might be still reading the stdout, so we shouldn't close the pipes here
		// MakeStdoutPipe returns a closer function to force callers to close the pipe correctly
		// Here we only need to mark the command as finished
		c.cmdFinished()
	}()

	if c.opts.PipelineFunc != nil {
		errPipeline := c.opts.PipelineFunc(&cmdContext{Context: c.cmdCtx, cmd: c})

		if context.Cause(c.cmdCtx) == nil {
			// if the context is not canceled explicitly, we need to discard the unread data,
			// and wait for the command to exit normally, and then get its exit code
			c.discardPipeReaders(c.parentPipeReaders)
		} // else: canceled command will be killed, and the exit code is caused by kill

		// after the pipeline function returns, we can safely close the pipes, then wait for the command to exit
		c.closePipeFiles(c.parentPipeFiles)
		errWait := c.cmd.Wait()
		errCause := context.Cause(c.cmdCtx) // in case the cause is set during Wait(), get the final cancel cause

		if unwrapped, ok := UnwrapPipelineError(errCause); ok {
			if unwrapped != errPipeline {
				panic("unwrapped context pipeline error should be the same one returned by pipeline function")
			}
			if unwrapped == nil {
				// the pipeline function declares that there is no error, and it cancels (kills) the command ahead,
				// so we should ignore the errors from "wait" and "cause"
				errWait, errCause = nil, nil
			}
		}

		// some legacy code still need to access the error returned by pipeline function by "==" but not "errors.Is"
		// so we need to make sure the original error is able to be unwrapped by UnwrapPipelineError
		return errors.Join(wrapPipelineError(errPipeline), errCause, errWait)
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
	if c.cmdStderr != nil {
		panic("caller-provided stderr receiver doesn't work with managed stderr buffer")
	}
	c.cmdManagedStderr = &bytes.Buffer{}
	c.cmdStderr = c.cmdManagedStderr
	err := c.Start(ctx)
	if err != nil {
		return &runStdError{err: err}
	}
	return nil
}

func (c *Command) WaitWithStderr() RunStdError {
	if c.cmdManagedStderr == nil {
		panic("managed stderr buffer is not initialized")
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
	if c.cmdStdout != nil || c.cmdStderr != nil {
		// it must panic here, otherwise there would be bugs if developers set other Stdin/Stderr by mistake, and it would be very difficult to debug
		panic("stdout and stderr field must be nil when using RunStdBytes")
	}
	stdoutBuf := &bytes.Buffer{}
	err := c.WithParentCallerInfo().WithStdoutBuffer(stdoutBuf).RunWithStderr(ctx)
	return stdoutBuf.Bytes(), c.cmdManagedStderr.Bytes(), err
}

func (c *Command) DebugKill() {
	_ = c.cmd.Process.Kill()
}
