// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Unknwon/com"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	commitCache *ObjectCache
	tagCache    *ObjectCache
}

const prettyLogFormat = `--pretty=format:%H`

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
	if err != nil {
		return false
	}
	return true
}

// InitRepository initializes a new Git repository.
func InitRepository(repoPath string, bare bool) error {
	os.MkdirAll(repoPath, os.ModePerm)

	cmd := NewCommand("init")
	if bare {
		cmd.AddArguments("--bare")
	}
	_, err := cmd.RunInDir(repoPath)
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

	return &Repository{
		Path:        repoPath,
		commitCache: newObjectCache(),
		tagCache:    newObjectCache(),
	}, nil
}

// CloneRepoOptions options when clone a repository
type CloneRepoOptions struct {
	Timeout time.Duration
	Mirror  bool
	Bare    bool
	Quiet   bool
	Branch  string
}

// Clone clones original repository to target path.
func Clone(from, to string, opts CloneRepoOptions) (err error) {
	toDir := path.Dir(to)
	if err = os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := NewCommand("clone")
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	if opts.Bare {
		cmd.AddArguments("--bare")
	}
	if opts.Quiet {
		cmd.AddArguments("--quiet")
	}
	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b", opts.Branch)
	}
	cmd.AddArguments(from, to)

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
		cmd.AddArguments(opts.Remote)
		cmd.AddArguments(opts.Branch)
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
}

// Push pushs local commits to given remote branch.
func Push(repoPath string, opts PushOptions) error {
	cmd := NewCommand("push")
	if opts.Force {
		cmd.AddArguments("-f")
	}
	cmd.AddArguments(opts.Remote, opts.Branch)
	_, err := cmd.RunInDir(repoPath)
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

// GetRepoSize returns disk consumption for repo in path
func GetRepoSize(repoPath string) (*CountObject, error) {
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
	return time.Parse("Mon Jan 02 15:04:05 2006 -0700", commitTime)
}
