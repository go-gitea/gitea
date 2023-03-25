// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

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
	if !filepath.IsAbs(opts.Dir) {
		cmd.Dir = filepath.Join(setting.RepoRootPath, opts.Dir)
	}
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

func RunServCommand(ctx context.Context, verb, repoPath string, envs []string) error {
	var gitcmd *exec.Cmd
	gitBinPath := filepath.Dir(gitExecutable)     // e.g. /usr/bin
	gitBinVerb := filepath.Join(gitBinPath, verb) // e.g. /usr/bin/git-upload-pack
	if _, err := os.Stat(gitBinVerb); err != nil {
		// if the command "git-upload-pack" doesn't exist, try to split "git-upload-pack" to use the sub-command with git
		// ps: Windows only has "git.exe" in the bin path, so Windows always uses this way
		verbFields := strings.SplitN(verb, "-", 2)
		if len(verbFields) == 2 {
			// use git binary with the sub-command part: "C:\...\bin\git.exe", "upload-pack", ...
			gitcmd = exec.CommandContext(ctx, gitExecutable, verbFields[1], repoPath)
		}
	}
	if gitcmd == nil {
		// by default, use the verb (it has been checked above by allowedCommands)
		gitcmd = exec.CommandContext(ctx, gitBinVerb, repoPath)
	}

	process.SetSysProcAttribute(gitcmd)
	gitcmd.Dir = setting.RepoRootPath
	gitcmd.Stdout = os.Stdout
	gitcmd.Stdin = os.Stdin
	gitcmd.Stderr = os.Stderr
	gitcmd.Env = append(gitcmd.Env, os.Environ()...)
	gitcmd.Env = append(gitcmd.Env, envs...)

	// to avoid breaking, here only use the minimal environment variables for the "gitea serv" command.
	// it could be re-considered whether to use the same git.CommonGitCmdEnvs() as "git" command later.
	gitcmd.Env = append(gitcmd.Env, CommonCmdServEnvs()...)

	return gitcmd.Run()
}
