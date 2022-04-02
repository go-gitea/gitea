// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

// LocalCommand represents a local git command
type LocalCommand struct {
	service          *LocalService
	args             []string
	parentContext    context.Context
	desc             string
	globalArgsLength int
}

var _ Command = &LocalCommand{}

func (c *LocalCommand) String() string {
	if len(c.args) == 0 {
		return c.service.GitExecutable
	}
	return fmt.Sprintf("%s %s", c.service.GitExecutable, strings.Join(c.args, " "))
}

// SetParentContext sets the parent context for this command
func (c *LocalCommand) SetParentContext(ctx context.Context) Command {
	c.parentContext = ctx
	return c
}

// SetDescription sets the description for this command which be returned on
// c.String()
func (c *LocalCommand) SetDescription(desc string) Command {
	c.desc = desc
	return c
}

// AddArguments adds new argument(s) to the command.
func (c *LocalCommand) AddArguments(args ...string) Command {
	c.args = append(c.args, args...)
	return c
}

// defaultLocale is the default LC_ALL to run git commands in.
const defaultLocale = "C"

// Run runs the command with the RunOpts
func (c *LocalCommand) Run(opts *RunOpts) error {
	if opts == nil {
		opts = &RunOpts{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = c.service.defaultTimeout
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
		desc = fmt.Sprintf("%s %s [repo_path: %s]", c.args[0], strings.Join(args, " "), opts.Dir)
	}
	desc = fmt.Sprintf("[%s] %s", c.service.GitExecutable, desc)

	ctx, cancel, finished := process.GetManager().AddContextTimeout(c.parentContext, opts.Timeout, desc)
	defer finished()

	cmd := exec.CommandContext(ctx, c.service.GitExecutable, c.args...)
	if opts.Env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = opts.Env
	}

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("LC_ALL=%s", defaultLocale),
		// avoid prompting for credentials interactively, supported since git v2.3
		"GIT_TERMINAL_PROMPT=0",
		// ignore replace references (https://git-scm.com/docs/git-replace)
		"GIT_NO_REPLACE_OBJECTS=1",
	)

	process.SetSysProcAttribute(cmd)
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

// defaultGitExecutable is the command name of git
// Could be updated to an absolute path while initialization
const defaultGitExecutable = "git"

// LocalService represents a command service to create local git commands
type LocalService struct {
	GitExecutable  string // git binary location
	RepoRootPath   string // repository storage root directory
	defaultTimeout time.Duration
}

var _ Service = &LocalService{}

// NewLocalService returns a local service
func NewLocalService(gitExecutable, repoRootPath string, defaultTimeout time.Duration) *LocalService {
	// If path is empty, we use the default value of GitExecutable "git" to search for the location of git.
	if gitExecutable == "" {
		gitExecutable = defaultGitExecutable
	}
	absPath, err := exec.LookPath(gitExecutable)
	if err != nil {
		panic(fmt.Sprintf("Git not found: %v", err))
	}

	return &LocalService{
		GitExecutable:  absPath,
		RepoRootPath:   repoRootPath,
		defaultTimeout: defaultTimeout,
	}
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
func (s *LocalService) NewCommand(ctx context.Context, gloablArgsLength int, args ...string) Command {
	return &LocalCommand{
		service:          s,
		args:             args,
		parentContext:    ctx,
		globalArgsLength: gloablArgsLength,
	}
}
