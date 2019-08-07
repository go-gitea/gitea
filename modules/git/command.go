// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/process"
)

var (
	// GlobalCommandArgs global command args for external package setting
	GlobalCommandArgs []string

	// DefaultCommandExecutionTimeout default command execution timeout duration
	DefaultCommandExecutionTimeout = 60 * time.Second
)

// Command represents a command with its subcommands or arguments.
type Command struct {
	name string
	args []string
}

func (c *Command) String() string {
	if len(c.args) == 0 {
		return c.name
	}
	return fmt.Sprintf("%s %s", c.name, strings.Join(c.args, " "))
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
func NewCommand(args ...string) *Command {
	// Make an explicit copy of GlobalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return &Command{
		name: GitExecutable,
		args: append(cargs, args...),
	}
}

// AddArguments adds new argument(s) to the command.
func (c *Command) AddArguments(args ...string) *Command {
	c.args = append(c.args, args...)
	return c
}

// RunInDirTimeoutEnvPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirTimeoutEnvPipeline(env []string, timeout time.Duration, dir string, stdout, stderr io.Writer) error {
	return c.RunInDirTimeoutEnvFullPipeline(env, timeout, dir, stdout, stderr, nil)
}

// RunInDirTimeoutEnvFullPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer and passes in an io.Reader as stdin.
func (c *Command) RunInDirTimeoutEnvFullPipeline(env []string, timeout time.Duration, dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	if timeout == -1 {
		timeout = DefaultCommandExecutionTimeout
	}

	if len(dir) == 0 {
		log(c.String())
	} else {
		log("%s: %v", dir, c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	pid := process.GetManager().Add(fmt.Sprintf("%s %s %s [repo_path: %s]", GitExecutable, c.name, strings.Join(c.args, " "), dir), cmd)
	defer process.GetManager().Remove(pid)

	if err := cmd.Wait(); err != nil {
		return err
	}

	return ctx.Err()
}

// RunInDirTimeoutPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirTimeoutPipeline(timeout time.Duration, dir string, stdout, stderr io.Writer) error {
	return c.RunInDirTimeoutEnvPipeline(nil, timeout, dir, stdout, stderr)
}

// RunInDirTimeoutFullPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer, and stdin from the given io.Reader
func (c *Command) RunInDirTimeoutFullPipeline(timeout time.Duration, dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	return c.RunInDirTimeoutEnvFullPipeline(nil, timeout, dir, stdout, stderr, stdin)
}

// RunInDirTimeout executes the command in given directory with given timeout,
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirTimeout(timeout time.Duration, dir string) ([]byte, error) {
	return c.RunInDirTimeoutEnv(nil, timeout, dir)
}

// RunInDirTimeoutEnv executes the command in given directory with given timeout,
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirTimeoutEnv(env []string, timeout time.Duration, dir string) ([]byte, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := c.RunInDirTimeoutEnvPipeline(env, timeout, dir, stdout, stderr); err != nil {
		return nil, concatenateError(err, stderr.String())
	}

	if stdout.Len() > 0 {
		log("stdout:\n%s", stdout.Bytes()[:1024])
	}
	return stdout.Bytes(), nil
}

// RunInDirPipeline executes the command in given directory,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirPipeline(dir string, stdout, stderr io.Writer) error {
	return c.RunInDirFullPipeline(dir, stdout, stderr, nil)
}

// RunInDirFullPipeline executes the command in given directory,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirFullPipeline(dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	return c.RunInDirTimeoutFullPipeline(-1, dir, stdout, stderr, stdin)
}

// RunInDirBytes executes the command in given directory
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirBytes(dir string) ([]byte, error) {
	return c.RunInDirTimeout(-1, dir)
}

// RunInDir executes the command in given directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunInDir(dir string) (string, error) {
	return c.RunInDirWithEnv(dir, nil)
}

// RunInDirWithEnv executes the command in given directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunInDirWithEnv(dir string, env []string) (string, error) {
	stdout, err := c.RunInDirTimeoutEnv(env, -1, dir)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

// RunTimeout executes the command in default working directory with given timeout,
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunTimeout(timeout time.Duration) (string, error) {
	stdout, err := c.RunInDirTimeout(timeout, "")
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

// Run executes the command in default working directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) Run() (string, error) {
	return c.RunTimeout(-1)
}
