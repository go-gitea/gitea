// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/proxy"
)

// GPGSettings represents the default GPG settings for this repository
type GPGSettings struct {
	Sign             bool
	KeyID            string
	Email            string
	Name             string
	PublicKeyContent string
}

const prettyLogFormat = `--pretty=format:%H`

// GetAllCommitsCount returns count of all commits in repository
func (repo *Repository) GetAllCommitsCount() (int64, error) {
	return AllCommitsCount(repo.Ctx, repo.Path, false)
}

func (repo *Repository) parsePrettyFormatLogToList(logs []byte) ([]*Commit, error) {
	var commits []*Commit
	if len(logs) == 0 {
		return commits, nil
	}

	parts := bytes.Split(logs, []byte{'\n'})

	for _, commitID := range parts {
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
	_, err := NewCommand(ctx, "ls-remote", "-q", "-h", url, "HEAD").Run()
	return err == nil
}

// InitRepository initializes a new Git repository.
func InitRepository(ctx context.Context, repoPath string, bare bool) error {
	err := os.MkdirAll(repoPath, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := NewCommand(ctx, "init")
	if bare {
		cmd.AddArguments("--bare")
	}
	_, err = cmd.RunInDir(repoPath)
	return err
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	var errbuf, output strings.Builder
	if err := NewCommand(repo.Ctx, "rev-list", "--all", "--count", "--max-count=1").
		RunWithContext(&RunContext{
			Timeout: -1,
			Dir:     repo.Path,
			Stdout:  &output,
			Stderr:  &errbuf,
		}); err != nil {
		return true, fmt.Errorf("check empty: %v - %s", err, errbuf.String())
	}

	c, err := strconv.Atoi(strings.TrimSpace(output.String()))
	if err != nil {
		return true, fmt.Errorf("check empty: convert %s to count failed: %v", output.String(), err)
	}
	return c == 0, nil
}

// CloneRepoOptions options when clone a repository
type CloneRepoOptions struct {
	Timeout    time.Duration
	Mirror     bool
	Bare       bool
	Quiet      bool
	Branch     string
	Shared     bool
	NoCheckout bool
	Depth      int
	Filter     string
}

// Clone clones original repository to target path.
func Clone(ctx context.Context, from, to string, opts CloneRepoOptions) error {
	cargs := make([]string, len(globalCommandArgs))
	copy(cargs, globalCommandArgs)
	return CloneWithArgs(ctx, from, to, cargs, opts)
}

// CloneWithArgs original repository to target path.
func CloneWithArgs(ctx context.Context, from, to string, args []string, opts CloneRepoOptions) (err error) {
	toDir := path.Dir(to)
	if err = os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := NewCommandContextNoGlobals(ctx, args...).AddArguments("clone")
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
		cmd.AddArguments("--depth", strconv.Itoa(opts.Depth))
	}
	if opts.Filter != "" {
		cmd.AddArguments("--filter", opts.Filter)
	}
	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b", opts.Branch)
	}
	cmd.AddArguments("--", from, to)

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	envs := os.Environ()
	u, err := url.Parse(from)
	if err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) {
		if proxy.Match(u.Host) {
			envs = append(envs, fmt.Sprintf("https_proxy=%s", proxy.GetProxyURL()))
		}
	}

	stderr := new(bytes.Buffer)
	if err = cmd.RunWithContext(&RunContext{
		Timeout: opts.Timeout,
		Env:     envs,
		Stdout:  io.Discard,
		Stderr:  stderr,
	}); err != nil {
		return ConcatenateError(err, stderr.String())
	}
	return nil
}

// PushOptions options when push to remote
type PushOptions struct {
	Remote  string
	Branch  string
	Force   bool
	Mirror  bool
	Env     []string
	Timeout time.Duration
}

// Push pushs local commits to given remote branch.
func Push(ctx context.Context, repoPath string, opts PushOptions) error {
	cmd := NewCommand(ctx, "push")
	if opts.Force {
		cmd.AddArguments("-f")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	cmd.AddArguments("--", opts.Remote)
	if len(opts.Branch) > 0 {
		cmd.AddArguments(opts.Branch)
	}
	var outbuf, errbuf strings.Builder

	if opts.Timeout == 0 {
		opts.Timeout = -1
	}

	err := cmd.RunWithContext(&RunContext{
		Env:     opts.Env,
		Timeout: opts.Timeout,
		Dir:     repoPath,
		Stdout:  &outbuf,
		Stderr:  &errbuf,
	})
	if err != nil {
		if strings.Contains(errbuf.String(), "non-fast-forward") {
			return &ErrPushOutOfDate{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(errbuf.String(), "! [remote rejected]") {
			err := &ErrPushRejected{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return err
		} else if strings.Contains(errbuf.String(), "matches more than one") {
			err := &ErrMoreThanOne{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			return err
		}
	}

	if errbuf.Len() > 0 && err != nil {
		return fmt.Errorf("%v - %s", err, errbuf.String())
	}

	return err
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func GetLatestCommitTime(ctx context.Context, repoPath string) (time.Time, error) {
	cmd := NewCommand(ctx, "for-each-ref", "--sort=-committerdate", BranchPrefix, "--count", "1", "--format=%(committerdate)")
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse(GitTimeLayout, commitTime)
}

// DivergeObject represents commit count diverging commits
type DivergeObject struct {
	Ahead  int
	Behind int
}

func checkDivergence(ctx context.Context, repoPath, baseBranch, targetBranch string) (int, error) {
	branches := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := NewCommand(ctx, "rev-list", "--count", branches)
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return -1, err
	}
	outInteger, errInteger := strconv.Atoi(strings.Trim(stdout, "\n"))
	if errInteger != nil {
		return -1, errInteger
	}
	return outInteger, nil
}

// GetDivergingCommits returns the number of commits a targetBranch is ahead or behind a baseBranch
func GetDivergingCommits(ctx context.Context, repoPath, baseBranch, targetBranch string) (DivergeObject, error) {
	// $(git rev-list --count master..feature) commits ahead of master
	ahead, errorAhead := checkDivergence(ctx, repoPath, baseBranch, targetBranch)
	if errorAhead != nil {
		return DivergeObject{}, errorAhead
	}

	// $(git rev-list --count feature..master) commits behind master
	behind, errorBehind := checkDivergence(ctx, repoPath, targetBranch, baseBranch)
	if errorBehind != nil {
		return DivergeObject{}, errorBehind
	}

	return DivergeObject{ahead, behind}, nil
}

// CreateBundle create bundle content to the target path
func (repo *Repository) CreateBundle(ctx context.Context, commit string, out io.Writer) error {
	tmp, err := os.MkdirTemp(os.TempDir(), "gitea-bundle")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	env := append(os.Environ(), "GIT_OBJECT_DIRECTORY="+filepath.Join(repo.Path, "objects"))
	_, err = NewCommand(ctx, "init", "--bare").RunInDirWithEnv(tmp, env)
	if err != nil {
		return err
	}

	_, err = NewCommand(ctx, "reset", "--soft", commit).RunInDirWithEnv(tmp, env)
	if err != nil {
		return err
	}

	_, err = NewCommand(ctx, "branch", "-m", "bundle").RunInDirWithEnv(tmp, env)
	if err != nil {
		return err
	}

	tmpFile := filepath.Join(tmp, "bundle")
	_, err = NewCommand(ctx, "bundle", "create", tmpFile, "bundle", "HEAD").RunInDirWithEnv(tmp, env)
	if err != nil {
		return err
	}

	fi, err := os.Open(tmpFile)
	if err != nil {
		return err
	}
	defer fi.Close()

	_, err = io.Copy(out, fi)
	return err
}
