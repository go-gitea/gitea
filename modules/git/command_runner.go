// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

type CommandRunner interface {
	String() string
	Run(ctx context.Context, desc string, args []string, opts *RunOpts) error
}

// localCommandRunner represents a command with its subcommands or arguments.
type localCommandRunner struct {
	name             string
	globalArgsLength int
}

var _ CommandRunner = &localCommandRunner{}

func newLocalCommandRunner(ctx context.Context) *localCommandRunner {
	return &localCommandRunner{
		name:             GitExecutable,
		globalArgsLength: len(globalCommandArgs),
	}
}

func (c *localCommandRunner) String() string {
	return c.name
}

// Run runs the command with the RunOpts
func (c *localCommandRunner) Run(ctx context.Context, desc string, args []string, opts *RunOpts) error {
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

	if desc == "" {
		args := args[c.globalArgsLength:]
		var argSensitiveURLIndexes []int
		for i, arg := range args {
			if strings.Contains(arg, "://") && strings.Contains(arg, "@") {
				argSensitiveURLIndexes = append(argSensitiveURLIndexes, i)
			}
		}
		if len(argSensitiveURLIndexes) > 0 {
			args = make([]string, len(args))
			copy(args, args)
			for _, urlArgIndex := range argSensitiveURLIndexes {
				args[urlArgIndex] = util.SanitizeCredentialURLs(args[urlArgIndex])
			}
		}
		desc = fmt.Sprintf("%s %s [repo_path: %s]", c.name, strings.Join(args, " "), opts.Dir)
	}

	var cancel context.CancelFunc
	var finished context.CancelFunc

	if opts.UseContextTimeout {
		ctx, cancel, finished = process.GetManager().AddContext(ctx, desc)
	} else {
		ctx, cancel, finished = process.GetManager().AddContextTimeout(ctx, timeout, desc)
	}
	defer finished()

	cmd := exec.CommandContext(ctx, c.name, args...)
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
