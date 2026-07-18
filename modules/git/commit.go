// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

// Commit represents a git commit.
type Commit struct {
	CommitMessage

	ID        ObjectID
	TreeID    ObjectID
	Parents   []ObjectID
	Author    *Signature // never nil
	Committer *Signature // never nil
	Signature *CommitSignature

	submoduleCache *ObjectCache[*SubModule]
	treeCache      *Tree
}

// CommitSignature represents a git commit signature part.
type CommitSignature struct {
	Signature string
	Payload   string
}

// ParentID returns oid of n-th parent (0-based index).
// It returns nil if no such parent exists.
func (c *Commit) ParentID(n int) (ObjectID, error) {
	if n >= len(c.Parents) {
		return nil, ErrNotExist{"", ""}
	}
	return c.Parents[n], nil
}

// Parent returns n-th parent (0-based index) of the commit.
func (c *Commit) Parent(ctx context.Context, gitRepo *Repository, n int) (*Commit, error) {
	id, err := c.ParentID(n)
	if err != nil {
		return nil, err
	}
	parent, err := gitRepo.getCommit(ctx, id)
	if err != nil {
		return nil, err
	}
	return parent, nil
}

// ParentCount returns number of parents of the commit.
// 0 if this is the root commit,  otherwise 1,2, etc.
func (c *Commit) ParentCount() int {
	return len(c.Parents)
}

// GetCommitByPath return the commit of relative path object.
func (c *Commit) GetCommitByPath(ctx context.Context, gitRepo *Repository, relpath string) (*Commit, error) {
	if gitRepo.LastCommitCache != nil {
		return gitRepo.LastCommitCache.GetCommitByPath(ctx, c.ID.String(), relpath)
	}
	return gitRepo.getCommitByPathWithID(ctx, c.ID, relpath)
}

func (c *Commit) Tree() *Tree {
	if c.treeCache == nil {
		c.treeCache = newTree(c.TreeID)
	}
	return c.treeCache
}

func (c *Commit) GetBlobByPath(ctx context.Context, gitRepo *Repository, relpath string) (*Blob, error) {
	return c.Tree().GetBlobByPath(ctx, gitRepo, relpath)
}

func (c *Commit) GetTreeEntryByPath(ctx context.Context, gitRepo *Repository, relpath string) (_ *TreeEntry, err error) {
	return c.Tree().GetTreeEntryByPath(ctx, gitRepo, relpath)
}

func (c *Commit) SubTree(ctx context.Context, gitRepo *Repository, relpath string) (*Tree, error) {
	return c.Tree().SubTree(ctx, gitRepo, relpath)
}

// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
func (c *Commit) CommitsByRange(ctx context.Context, gitRepo *Repository, page, pageSize int, not, since, until string) ([]*Commit, error) {
	return gitRepo.commitsByRangeWithTime(ctx, c.ID, page, pageSize, not, since, until)
}

// CommitsBefore returns all the commits before current revision
func (c *Commit) CommitsBefore(ctx context.Context, gitRepo *Repository) ([]*Commit, error) {
	return gitRepo.getCommitsBefore(ctx, c.ID)
}

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(ctx context.Context, gitRepo *Repository, objectID ObjectID) (bool, error) {
	this := c.ID.String()
	that := objectID.String()

	if this == that {
		return false, nil
	}

	_, _, err := gitcmd.NewCommand("merge-base", "--is-ancestor").
		AddDynamicArguments(that, this).
		WithDir(gitRepo.Path).
		RunStdString(ctx)
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if exitError.ProcessState.ExitCode() == 1 && len(exitError.Stderr) == 0 {
			return false, nil
		}
	}
	return false, err
}

// IsForcePush returns true if a push from oldCommitHash to this is a force push
func (c *Commit) IsForcePush(ctx context.Context, gitRepo *Repository, oldCommitID string) (bool, error) {
	objectFormat, err := gitRepo.GetObjectFormat(ctx)
	if err != nil {
		return false, err
	}
	if oldCommitID == objectFormat.EmptyObjectID().String() {
		return false, nil
	}

	oldCommit, err := gitRepo.GetCommit(ctx, oldCommitID)
	if err != nil {
		return false, err
	}
	hasPreviousCommit, err := c.HasPreviousCommit(ctx, gitRepo, oldCommit.ID)
	return !hasPreviousCommit, err
}

// CommitsBeforeLimit returns num commits before current revision
func (c *Commit) CommitsBeforeLimit(ctx context.Context, gitRepo *Repository, num int) ([]*Commit, error) {
	return gitRepo.getCommitsBeforeLimit(ctx, c.ID, num)
}

// CommitsBeforeUntil returns the commits in range "[cur, ref)"
func (c *Commit) CommitsBeforeUntil(ctx context.Context, gitRepo *Repository, ref RefName) ([]*Commit, error) {
	return gitRepo.CommitsBetween(ctx, c.ID.RefName(), ref, -1)
}

// SearchCommitsOptions specify the parameters for SearchCommits
type SearchCommitsOptions struct {
	Keywords            []string
	Authors, Committers []string
	After, Before       string
	All                 bool
}

// NewSearchCommitsOptions construct a SearchCommitsOption from a space-delimited search string
func NewSearchCommitsOptions(searchString string, forAllRefs bool) SearchCommitsOptions {
	var keywords, authors, committers []string
	var after, before string

	fields := strings.FieldsSeq(searchString)
	for k := range fields {
		switch {
		case strings.HasPrefix(k, "author:"):
			authors = append(authors, strings.TrimPrefix(k, "author:"))
		case strings.HasPrefix(k, "committer:"):
			committers = append(committers, strings.TrimPrefix(k, "committer:"))
		case strings.HasPrefix(k, "after:"):
			after = strings.TrimPrefix(k, "after:")
		case strings.HasPrefix(k, "before:"):
			before = strings.TrimPrefix(k, "before:")
		default:
			keywords = append(keywords, k)
		}
	}

	return SearchCommitsOptions{
		Keywords:   keywords,
		Authors:    authors,
		Committers: committers,
		After:      after,
		Before:     before,
		All:        forAllRefs,
	}
}

// SearchCommits returns the commits match the keyword before current revision
func (c *Commit) SearchCommits(ctx context.Context, gitRepo *Repository, opts SearchCommitsOptions) ([]*Commit, error) {
	return gitRepo.searchCommits(ctx, c.ID, opts)
}

// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
func (c *Commit) GetFilesChangedSinceCommit(ctx context.Context, gitRepo *Repository, pastCommit string) ([]string, error) {
	return gitRepo.GetFilesChangedBetween(ctx, pastCommit, c.ID.String())
}

// FileChangedSinceCommit Returns true if the file given has changed since the past commit
// YOU MUST ENSURE THAT pastCommit is a valid commit ID.
func (c *Commit) FileChangedSinceCommit(ctx context.Context, gitRepo *Repository, filename, pastCommit string) (bool, error) {
	return gitRepo.FileChangedBetweenCommits(ctx, filename, pastCommit, c.ID.String())
}

// GetFileContent reads a file content as a string or returns false if this was not possible
func (c *Commit) GetFileContent(ctx context.Context, gitRepo *Repository, filename string, limit int) (string, error) {
	entry, err := c.GetTreeEntryByPath(ctx, gitRepo, filename)
	if err != nil {
		return "", err
	}

	r, err := entry.Blob(gitRepo).DataAsync(ctx)
	if err != nil {
		return "", err
	}
	defer r.Close()

	if limit > 0 {
		bs := make([]byte, limit)
		n, err := util.ReadAtMost(r, bs)
		if err != nil {
			return "", err
		}
		return string(bs[:n]), nil
	}

	bytes, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
func GetFullCommitID(ctx context.Context, repoPath, shortID string) (string, error) {
	commitID, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(shortID).
		WithDir(repoPath).
		RunStdString(ctx)
	if err != nil {
		if gitcmd.IsErrorExitCode(err, 128) {
			return "", ErrNotExist{shortID, ""}
		}
		return "", err
	}
	return strings.TrimSpace(commitID), nil
}

func IsStringLikelyCommitID(objFmt ObjectFormat, s string, minLength ...int) bool {
	maxLen := 64 // sha256
	if objFmt != nil {
		maxLen = objFmt.FullLength()
	}
	minLen := util.OptionalArg(minLength, maxLen)
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	return isStringLowerHex(s)
}

func isStringLowerHex(s string) bool {
	for _, c := range s {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			return false
		}
	}
	return len(s) > 0 // it accepts odd length because "shorten commit id" can be 7-chars
}
