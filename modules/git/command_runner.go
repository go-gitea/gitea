// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"code.gitea.io/gitea/modules/process"

	"github.com/hashicorp/go-version"
)

// gitExecutable is the command name of git
// Could be updated to an absolute path while initialization
var gitExecutable = "git"

// SetExecutablePath changes the path of git executable and checks the file permission and version.
func SetExecutablePath(path string) error {
	// If path is empty, we use the default value of gitExecutable "git" to search for the location of git.
	if path != "" {
		gitExecutable = path
	}
	absPath, err := exec.LookPath(gitExecutable)
	if err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	gitExecutable = absPath

	_, err = loadGitVersion()
	if err != nil {
		return fmt.Errorf("unable to load git version: %w", err)
	}

	versionRequired, err := version.NewVersion(RequiredVersion)
	if err != nil {
		return err
	}

	if gitVersion.LessThan(versionRequired) {
		moreHint := "get git: https://git-scm.com/download/"
		if runtime.GOOS == "linux" {
			// there are a lot of CentOS/RHEL users using old git, so we add a special hint for them
			if _, err = os.Stat("/etc/redhat-release"); err == nil {
				// ius.io is the recommended official(git-scm.com) method to install git
				moreHint = "get git: https://git-scm.com/download/linux and https://ius.io"
			}
		}
		return fmt.Errorf("installed git version %q is not supported, Gitea requires git version >= %q, %s", gitVersion.Original(), RequiredVersion, moreHint)
	}

	return nil
}

type CommandRunner interface {
	String() string
	Run(ctx context.Context, args []string, opts *RunOpts, cancel context.CancelFunc) error
}

// localCommandRunner represents a command with its subcommands or arguments.
type localCommandRunner struct {
	name string
}

var _ CommandRunner = &localCommandRunner{}

func newLocalCommandRunner(ctx context.Context) *localCommandRunner {
	return &localCommandRunner{
		name: gitExecutable,
	}
}

func (c *localCommandRunner) String() string {
	return c.name
}

// Run runs the command with the RunOpts
func (c *localCommandRunner) Run(ctx context.Context, args []string, opts *RunOpts, cancel context.CancelFunc) error {
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
