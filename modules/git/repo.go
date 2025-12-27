// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/proxy"
)

const prettyLogFormat = `--pretty=format:%H`

func (repo *Repository) ShowPrettyFormatLogToList(ctx context.Context, revisionRange string) ([]*Commit, error) {
	// avoid: ambiguous argument 'refs/a...refs/b': unknown revision or path not in the working tree. Use '--': 'git <command> [<revision>...] -- [<file>...]'
	logs, _, err := gitcmd.NewCommand("log").AddArguments(prettyLogFormat).
		AddDynamicArguments(revisionRange).AddArguments("--").WithDir(repo.Path).
		RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(logs)
}

func (repo *Repository) parsePrettyFormatLogToList(logs []byte) ([]*Commit, error) {
	var commits []*Commit
	if len(logs) == 0 {
		return commits, nil
	}

	parts := bytes.SplitSeq(logs, []byte{'\n'})

	for commitID := range parts {
		commit, err := repo.GetCommit(string(commitID))
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

// IsRepoURLAccessible checks if given repository URL is accessible.
func IsRepoURLAccessible(ctx context.Context, url string) bool {
	_, _, err := gitcmd.NewCommand("ls-remote", "-q", "-h").AddDynamicArguments(url, "HEAD").RunStdString(ctx)
	return err == nil
}

// InitRepository initializes a new Git repository.
func InitRepository(ctx context.Context, repoPath string, bare bool, objectFormatName string) error {
	err := os.MkdirAll(repoPath, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := gitcmd.NewCommand("init")

	if !IsValidObjectFormat(objectFormatName) {
		return fmt.Errorf("invalid object format: %s", objectFormatName)
	}
	if DefaultFeatures().SupportHashSha256 {
		cmd.AddOptionValues("--object-format", objectFormatName)
	}

	if bare {
		cmd.AddArguments("--bare")
	}
	_, _, err = cmd.WithDir(repoPath).RunStdString(ctx)
	return err
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	var errbuf, output strings.Builder
	if err := gitcmd.NewCommand().
		AddOptionFormat("--git-dir=%s", repo.Path).
		AddArguments("rev-list", "-n", "1", "--all").
		WithDir(repo.Path).
		WithStdout(&output).
		WithStderr(&errbuf).
		Run(repo.Ctx); err != nil {
		if (err.Error() == "exit status 1" && strings.TrimSpace(errbuf.String()) == "") || err.Error() == "exit status 129" {
			// git 2.11 exits with 129 if the repo is empty
			return true, nil
		}
		return true, fmt.Errorf("check empty: %w - %s", err, errbuf.String())
	}

	return strings.TrimSpace(output.String()) == "", nil
}

// CloneRepoOptions options when clone a repository
type CloneRepoOptions struct {
	Timeout       time.Duration
	Mirror        bool
	Bare          bool
	Quiet         bool
	Branch        string
	Shared        bool
	NoCheckout    bool
	Depth         int
	Filter        string
	SkipTLSVerify bool
	SingleBranch  bool
	Env           []string
}

// Clone clones original repository to target path.
func Clone(ctx context.Context, from, to string, opts CloneRepoOptions) error {
	toDir := path.Dir(to)
	if err := os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := gitcmd.NewCommand().AddArguments("clone")
	if opts.SkipTLSVerify {
		cmd.AddArguments("-c", "http.sslVerify=false")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	if opts.Bare {
		cmd.AddArguments("--bare")
	}
	if opts.Quiet {
		cmd.AddArguments("--quiet")
	}
	if opts.Shared {
		cmd.AddArguments("-s")
	}
	if opts.NoCheckout {
		cmd.AddArguments("--no-checkout")
	}
	if opts.Depth > 0 {
		cmd.AddArguments("--depth").AddDynamicArguments(strconv.Itoa(opts.Depth))
	}
	if opts.Filter != "" {
		cmd.AddArguments("--filter").AddDynamicArguments(opts.Filter)
	}
	if opts.SingleBranch {
		cmd.AddArguments("--single-branch")
	}
	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b").AddDynamicArguments(opts.Branch)
	}
	cmd.AddDashesAndList(from, to)

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	envs := os.Environ()
	if opts.Env != nil {
		envs = opts.Env
	} else {
		u, err := url.Parse(from)
		if err == nil {
			envs = proxy.EnvWithProxy(u)
		}
	}

	stderr := new(bytes.Buffer)
	if err := cmd.
		WithTimeout(opts.Timeout).
		WithEnv(envs).
		WithStdout(io.Discard).
		WithStderr(stderr).
		Run(ctx); err != nil {
		return gitcmd.ConcatenateError(err, stderr.String())
	}
	return nil
}

// PushOptions options when push to remote
type PushOptions struct {
	Remote         string
	LocalRefName   string
	Branch         string
	Force          bool
	ForceWithLease string
	Mirror         bool
	Env            []string
	Timeout        time.Duration
}

// Push pushs local commits to given remote branch.
func Push(ctx context.Context, repoPath string, opts PushOptions) error {
	cmd := gitcmd.NewCommand("push")
	if opts.ForceWithLease != "" {
		cmd.AddOptionFormat("--force-with-lease=%s", opts.ForceWithLease)
	} else if opts.Force {
		cmd.AddArguments("-f")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	remoteBranchArgs := []string{opts.Remote}
	if len(opts.Branch) > 0 {
		var refspec string
		if opts.LocalRefName != "" {
			refspec = fmt.Sprintf("%s:%s", opts.LocalRefName, opts.Branch)
		} else {
			refspec = opts.Branch
		}
		remoteBranchArgs = append(remoteBranchArgs, refspec)
	}
	cmd.AddDashesAndList(remoteBranchArgs...)

	stdout, stderr, err := cmd.WithEnv(opts.Env).WithTimeout(opts.Timeout).WithDir(repoPath).RunStdString(ctx)
	if err != nil {
		if strings.Contains(stderr, "non-fast-forward") {
			return &ErrPushOutOfDate{StdOut: stdout, StdErr: stderr, Err: err}
		} else if strings.Contains(stderr, "! [remote rejected]") || strings.Contains(stderr, "! [rejected]") {
			err := &ErrPushRejected{StdOut: stdout, StdErr: stderr, Err: err}
			err.GenerateMessage()
			return err
		} else if strings.Contains(stderr, "matches more than one") {
			return &ErrMoreThanOne{StdOut: stdout, StdErr: stderr, Err: err}
		}
		return fmt.Errorf("push failed: %w - %s\n%s", err, stderr, stdout)
	}

	return nil
}
