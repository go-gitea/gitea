// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gitealog "code.gitea.io/gitea/modules/log"
	"github.com/unknwon/com"
	"gopkg.in/src-d/go-billy.v4/osfs"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	gogitRepo    *gogit.Repository
	gogitStorage *filesystem.Storage
	gpgSettings  *GPGSettings
}

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
	return AllCommitsCount(repo.Path)
}

func (repo *Repository) parsePrettyFormatLogToList(logs []byte) (*list.List, error) {
	l := list.New()
	if len(logs) == 0 {
		return l, nil
	}

	parts := bytes.Split(logs, []byte{'\n'})

	for _, commitID := range parts {
		commit, err := repo.GetCommit(string(commitID))
		if err != nil {
			return nil, err
		}
		l.PushBack(commit)
	}

	return l, nil
}

// IsRepoURLAccessible checks if given repository URL is accessible.
func IsRepoURLAccessible(url string) bool {
	_, err := NewCommand("ls-remote", "-q", "-h", url, "HEAD").Run()
	return err == nil
}

// InitRepository initializes a new Git repository.
func InitRepository(repoPath string, bare bool) error {
	err := os.MkdirAll(repoPath, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := NewCommand("init")
	if bare {
		cmd.AddArguments("--bare")
	}
	_, err = cmd.RunInDir(repoPath)
	return err
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
	}

	fs := osfs.New(repoPath)
	_, err = fs.Stat(".git")
	if err == nil {
		fs, err = fs.Chroot(".git")
		if err != nil {
			return nil, err
		}
	}
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
	gogitRepo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Path:         repoPath,
		gogitRepo:    gogitRepo,
		gogitStorage: storage,
		tagCache:     newObjectCache(),
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() {
	if repo == nil || repo.gogitStorage == nil {
		return
	}
	if err := repo.gogitStorage.Close(); err != nil {
		gitealog.Error("Error closing storage: %v", err)
	}
}

// GoGitRepo gets the go-git repo representation
func (repo *Repository) GoGitRepo() *gogit.Repository {
	return repo.gogitRepo
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	var errbuf strings.Builder
	if err := NewCommand("log", "-1").RunInDirPipeline(repo.Path, nil, &errbuf); err != nil {
		if strings.Contains(errbuf.String(), "fatal: bad default revision 'HEAD'") ||
			strings.Contains(errbuf.String(), "fatal: your current branch 'master' does not have any commits yet") {
			return true, nil
		}
		return true, fmt.Errorf("check empty: %v - %s", err, errbuf.String())
	}

	return false, nil
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
}

// Clone clones original repository to target path.
func Clone(from, to string, opts CloneRepoOptions) (err error) {
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return CloneWithArgs(from, to, cargs, opts)
}

// CloneWithArgs original repository to target path.
func CloneWithArgs(from, to string, args []string, opts CloneRepoOptions) (err error) {
	toDir := path.Dir(to)
	if err = os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := NewCommandNoGlobals(args...).AddArguments("clone")
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

	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b", opts.Branch)
	}
	cmd.AddArguments("--", from, to)

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	_, err = cmd.RunTimeout(opts.Timeout)
	return err
}

// PullRemoteOptions options when pull from remote
type PullRemoteOptions struct {
	Timeout time.Duration
	All     bool
	Rebase  bool
	Remote  string
	Branch  string
}

// Pull pulls changes from remotes.
func Pull(repoPath string, opts PullRemoteOptions) error {
	cmd := NewCommand("pull")
	if opts.Rebase {
		cmd.AddArguments("--rebase")
	}
	if opts.All {
		cmd.AddArguments("--all")
	} else {
		cmd.AddArguments("--", opts.Remote, opts.Branch)
	}

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	_, err := cmd.RunInDirTimeout(opts.Timeout, repoPath)
	return err
}

// PushOptions options when push to remote
type PushOptions struct {
	Remote string
	Branch string
	Force  bool
	Env    []string
}

// Push pushs local commits to given remote branch.
func Push(repoPath string, opts PushOptions) error {
	cmd := NewCommand("push")
	if opts.Force {
		cmd.AddArguments("-f")
	}
	cmd.AddArguments("--", opts.Remote, opts.Branch)
	_, err := cmd.RunInDirWithEnv(repoPath, opts.Env)
	return err
}

// CheckoutOptions options when heck out some branch
type CheckoutOptions struct {
	Timeout   time.Duration
	Branch    string
	OldBranch string
}

// Checkout checkouts a branch
func Checkout(repoPath string, opts CheckoutOptions) error {
	cmd := NewCommand("checkout")
	if len(opts.OldBranch) > 0 {
		cmd.AddArguments("-b")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	cmd.AddArguments(opts.Branch)

	if len(opts.OldBranch) > 0 {
		cmd.AddArguments(opts.OldBranch)
	}

	_, err := cmd.RunInDirTimeout(opts.Timeout, repoPath)
	return err
}

// ResetHEAD resets HEAD to given revision or head of branch.
func ResetHEAD(repoPath string, hard bool, revision string) error {
	cmd := NewCommand("reset")
	if hard {
		cmd.AddArguments("--hard")
	}
	_, err := cmd.AddArguments(revision).RunInDir(repoPath)
	return err
}

// MoveFile moves a file to another file or directory.
func MoveFile(repoPath, oldTreeName, newTreeName string) error {
	_, err := NewCommand("mv").AddArguments(oldTreeName, newTreeName).RunInDir(repoPath)
	return err
}

// CountObject represents repository count objects report
type CountObject struct {
	Count       int64
	Size        int64
	InPack      int64
	Packs       int64
	SizePack    int64
	PrunePack   int64
	Garbage     int64
	SizeGarbage int64
}

const (
	statCount        = "count: "
	statSize         = "size: "
	statInpack       = "in-pack: "
	statPacks        = "packs: "
	statSizePack     = "size-pack: "
	statPrunePackage = "prune-package: "
	statGarbage      = "garbage: "
	statSizeGarbage  = "size-garbage: "
)

// CountObjects returns the results of git count-objects on the repoPath
func CountObjects(repoPath string) (*CountObject, error) {
	cmd := NewCommand("count-objects", "-v")
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return nil, err
	}

	return parseSize(stdout), nil
}

// parseSize parses the output from count-objects and return a CountObject
func parseSize(objects string) *CountObject {
	repoSize := new(CountObject)
	for _, line := range strings.Split(objects, "\n") {
		switch {
		case strings.HasPrefix(line, statCount):
			repoSize.Count = com.StrTo(line[7:]).MustInt64()
		case strings.HasPrefix(line, statSize):
			repoSize.Size = com.StrTo(line[6:]).MustInt64() * 1024
		case strings.HasPrefix(line, statInpack):
			repoSize.InPack = com.StrTo(line[9:]).MustInt64()
		case strings.HasPrefix(line, statPacks):
			repoSize.Packs = com.StrTo(line[7:]).MustInt64()
		case strings.HasPrefix(line, statSizePack):
			repoSize.SizePack = com.StrTo(line[11:]).MustInt64() * 1024
		case strings.HasPrefix(line, statPrunePackage):
			repoSize.PrunePack = com.StrTo(line[16:]).MustInt64()
		case strings.HasPrefix(line, statGarbage):
			repoSize.Garbage = com.StrTo(line[9:]).MustInt64()
		case strings.HasPrefix(line, statSizeGarbage):
			repoSize.SizeGarbage = com.StrTo(line[14:]).MustInt64() * 1024
		}
	}
	return repoSize
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func GetLatestCommitTime(repoPath string) (time.Time, error) {
	cmd := NewCommand("for-each-ref", "--sort=-committerdate", "refs/heads/", "--count", "1", "--format=%(committerdate)")
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

func checkDivergence(repoPath string, baseBranch string, targetBranch string) (int, error) {
	branches := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := NewCommand("rev-list", "--count", branches)
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
func GetDivergingCommits(repoPath string, baseBranch string, targetBranch string) (DivergeObject, error) {
	// $(git rev-list --count master..feature) commits ahead of master
	ahead, errorAhead := checkDivergence(repoPath, baseBranch, targetBranch)
	if errorAhead != nil {
		return DivergeObject{}, errorAhead
	}

	// $(git rev-list --count feature..master) commits behind master
	behind, errorBehind := checkDivergence(repoPath, targetBranch, baseBranch)
	if errorBehind != nil {
		return DivergeObject{}, errorBehind
	}

	return DivergeObject{ahead, behind}, nil
}
