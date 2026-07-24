// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type RepositoryFacade = gitrepo.RepositoryFacade

type RepositoryBase struct {
	LastCommitCache *LastCommitCache

	repoFacade        RepositoryFacade
	tagCache          *ObjectCache[*Tag]
	objectFormatCache ObjectFormat

	mu                 sync.Mutex
	catFileBatchCloser CatFileBatchCloser
	catFileBatchInUse  bool
}

var _ RepositoryFacade = (*Repository)(nil)

func (repo *Repository) GitRepoManagedID() string {
	return repo.repoFacade.GitRepoManagedID()
}

func (repo *Repository) GitRepoLocation() string {
	return repo.repoFacade.GitRepoLocation()
}

func (repo *Repository) LogString() string {
	return repo.repoFacade.LogString()
}

func OpenRepository(repo RepositoryFacade) (*Repository, error) {
	repoPath := gitrepo.RepoLocalPath(repo)
	exist, err := util.IsDir(repoPath)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}
	gitRepo := &Repository{
		RepositoryBase: RepositoryBase{tagCache: newObjectCache[*Tag](), repoFacade: repo},
	}
	if err = openRepositoryInternal(gitRepo); err != nil {
		return nil, err
	}
	return gitRepo, nil
}

// OpenRepositoryLocal opens a local repository that is not managed by Gitea
// If the path is relative, it will be converted to an absolute path using filepath.Abs (base on current working path)
func OpenRepositoryLocal(localPath string) (_ *Repository, err error) {
	if !filepath.IsAbs(localPath) {
		localPath, err = filepath.Abs(localPath)
		if err != nil {
			return nil, err
		}
	}
	return OpenRepository(gitrepo.RepositoryUnmanaged(localPath))
}

func (repo *Repository) Close() error {
	if repo == nil {
		setting.PanicInDevOrTesting("don't close a nil repository")
		return nil
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.catFileBatchCloser != nil {
		repo.catFileBatchCloser.Close()
		repo.catFileBatchCloser = nil
		repo.catFileBatchInUse = false
	}
	return repo.closeInternal()
}

// IsRepoURLAccessible checks if given repository URL is accessible.
func IsRepoURLAccessible(ctx context.Context, url string) bool {
	_, _, err := gitcmd.NewCommand("ls-remote", "-q", "-h").AddDynamicArguments(url, "HEAD").RunStdString(ctx)
	return err == nil
}

// InitRepositoryLocal initializes a new Git repository.
func InitRepositoryLocal(ctx context.Context, localRepoPath string, bare bool, objectFormatName string) error {
	err := os.MkdirAll(localRepoPath, os.ModePerm)
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
	_, _, err = cmd.WithDir(localRepoPath).RunStdString(ctx)
	return err
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty(ctx context.Context) (bool, error) {
	stdout, _, err := gitcmd.NewCommand().
		AddOptionFormat("--git-dir=%s", gitrepo.RepoLocalPath(repo)). // TODO: all git commands should use "--git-dir" or "GIT_DIR=..."
		AddArguments("rev-list", "-n", "1", "--all").
		WithRepo(repo).
		RunStdString(ctx)
	if err != nil {
		if (gitcmd.IsErrorExitCode(err, 1) && err.Stderr() == "") || gitcmd.IsErrorExitCode(err, 129) {
			// git 2.11 exits with 129 if the repo is empty
			return true, nil
		}
		return true, fmt.Errorf("check empty: %w", err)
	}
	return strings.TrimSpace(stdout) == "", nil
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
	HandleGitCmdHTTPRedirection(cmd, from, to)
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

	return cmd.
		WithTimeout(opts.Timeout).
		WithEnv(envs).
		RunWithStderr(ctx)
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
func Push(ctx context.Context, localRepoPath string, opts PushOptions) error {
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

	stdout, stderr, err := cmd.WithEnv(opts.Env).WithTimeout(opts.Timeout).WithDir(localRepoPath).RunStdString(ctx)
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

// CatFileBatch obtains a "batch object provider" for this repository.
// It reuses an existing one if available, otherwise creates a new one.
func (repo *Repository) CatFileBatch(ctx context.Context) (_ CatFileBatch, closeFunc func(), err error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if repo.catFileBatchCloser != nil && !repo.catFileBatchInUse {
		if ctx != repo.catFileBatchCloser.Context() {
			repo.catFileBatchCloser.Close()
			repo.catFileBatchCloser = nil
			repo.catFileBatchInUse = false
		}
	}

	if repo.catFileBatchCloser == nil {
		repo.catFileBatchCloser, err = NewBatch(ctx, repo)
		if err != nil {
			repo.catFileBatchCloser = nil // otherwise it is "interface(nil)" and will cause wrong logic
			return nil, nil, err
		}
	}

	if !repo.catFileBatchInUse {
		repo.catFileBatchInUse = true
		return CatFileBatch(repo.catFileBatchCloser), func() {
			repo.mu.Lock()
			defer repo.mu.Unlock()
			repo.catFileBatchInUse = false
		}, nil
	}

	tempBatch, err := NewBatch(ctx, repo)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, tempBatch.Close, nil
}
