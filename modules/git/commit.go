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

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
)

// Commit represents a git commit.
type Commit struct {
	Tree // FIXME: bad design, this field can be nil if the commit is from "last commit cache"

	ID            ObjectID
	Author        *Signature // never nil
	Committer     *Signature // never nil
	CommitMessage string
	Signature     *CommitSignature

	Parents        []ObjectID // ID strings
	submoduleCache *ObjectCache[*SubModule]
}

// CommitSignature represents a git commit signature part.
type CommitSignature struct {
	Signature string
	Payload   string
}

// Message returns the commit message. Same as retrieving CommitMessage directly.
func (c *Commit) Message() string {
	return c.CommitMessage
}

// Summary returns first line of commit message.
// The string is forced to be valid UTF8
func (c *Commit) Summary() string {
	return strings.ToValidUTF8(strings.Split(strings.TrimSpace(c.CommitMessage), "\n")[0], "?")
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
func (c *Commit) Parent(n int) (*Commit, error) {
	id, err := c.ParentID(n)
	if err != nil {
		return nil, err
	}
	parent, err := c.repo.getCommit(id)
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
func (c *Commit) GetCommitByPath(relpath string) (*Commit, error) {
	if c.repo.LastCommitCache != nil {
		return c.repo.LastCommitCache.GetCommitByPath(c.ID.String(), relpath)
	}
	return c.repo.getCommitByPathWithID(c.ID, relpath)
}

// AddChanges marks local changes to be ready for commit.
func AddChanges(ctx context.Context, repoPath string, all bool, files ...string) error {
	cmd := gitcmd.NewCommand().AddArguments("add")
	if all {
		cmd.AddArguments("--all")
	}
	cmd.AddDashesAndList(files...)
	_, _, err := cmd.WithDir(repoPath).RunStdString(ctx)
	return err
}

// CommitChangesOptions the options when a commit created
type CommitChangesOptions struct {
	Committer *Signature
	Author    *Signature
	Message   string
}

// CommitChanges commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChanges(ctx context.Context, repoPath string, opts CommitChangesOptions) error {
	cmd := gitcmd.NewCommand()
	if opts.Committer != nil {
		cmd.AddOptionValues("-c", "user.name="+opts.Committer.Name)
		cmd.AddOptionValues("-c", "user.email="+opts.Committer.Email)
	}
	cmd.AddArguments("commit")

	if opts.Author == nil {
		opts.Author = opts.Committer
	}
	if opts.Author != nil {
		cmd.AddOptionFormat("--author='%s <%s>'", opts.Author.Name, opts.Author.Email)
	}
	cmd.AddOptionFormat("--message=%s", opts.Message)

	_, _, err := cmd.WithDir(repoPath).RunStdString(ctx)
	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
func (c *Commit) CommitsByRange(page, pageSize int, not, since, until string) ([]*Commit, error) {
	return c.repo.commitsByRangeWithTime(c.ID, page, pageSize, not, since, until)
}

// CommitsBefore returns all the commits before current revision
func (c *Commit) CommitsBefore() ([]*Commit, error) {
	return c.repo.getCommitsBefore(c.ID)
}

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(objectID ObjectID) (bool, error) {
	this := c.ID.String()
	that := objectID.String()

	if this == that {
		return false, nil
	}

	_, _, err := gitcmd.NewCommand("merge-base", "--is-ancestor").
		AddDynamicArguments(that, this).
		WithDir(c.repo.Path).
		RunStdString(c.repo.Ctx)
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
func (c *Commit) IsForcePush(oldCommitID string) (bool, error) {
	objectFormat, err := c.repo.GetObjectFormat()
	if err != nil {
		return false, err
	}
	if oldCommitID == objectFormat.EmptyObjectID().String() {
		return false, nil
	}

	oldCommit, err := c.repo.GetCommit(oldCommitID)
	if err != nil {
		return false, err
	}
	hasPreviousCommit, err := c.HasPreviousCommit(oldCommit.ID)
	return !hasPreviousCommit, err
}

// CommitsBeforeLimit returns num commits before current revision
func (c *Commit) CommitsBeforeLimit(num int) ([]*Commit, error) {
	return c.repo.getCommitsBeforeLimit(c.ID, num)
}

// CommitsBeforeUntil returns the commits between commitID to current revision
func (c *Commit) CommitsBeforeUntil(commitID string) ([]*Commit, error) {
	endCommit, err := c.repo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	return c.repo.CommitsBetween(c, endCommit)
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
func (c *Commit) SearchCommits(opts SearchCommitsOptions) ([]*Commit, error) {
	return c.repo.searchCommits(c.ID, opts)
}

// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
func (c *Commit) GetFilesChangedSinceCommit(pastCommit string) ([]string, error) {
	return c.repo.GetFilesChangedBetween(pastCommit, c.ID.String())
}

// FileChangedSinceCommit Returns true if the file given has changed since the past commit
// YOU MUST ENSURE THAT pastCommit is a valid commit ID.
func (c *Commit) FileChangedSinceCommit(filename, pastCommit string) (bool, error) {
	return c.repo.FileChangedBetweenCommits(filename, pastCommit, c.ID.String())
}

// HasFile returns true if the file given exists on this commit
// This does only mean it's there - it does not mean the file was changed during the commit.
func (c *Commit) HasFile(filename string) (bool, error) {
	_, err := c.GetBlobByPath(filename)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetFileContent reads a file content as a string or returns false if this was not possible
func (c *Commit) GetFileContent(filename string, limit int) (string, error) {
	entry, err := c.GetTreeEntryByPath(filename)
	if err != nil {
		return "", err
	}

	r, err := entry.Blob().DataAsync()
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

// GetBranchName gets the closest branch name (as returned by 'git name-rev --name-only')
func (c *Commit) GetBranchName() (string, error) {
	cmd := gitcmd.NewCommand("name-rev")
	if DefaultFeatures().CheckVersionAtLeast("2.13.0") {
		cmd.AddArguments("--exclude", "refs/tags/*")
	}
	cmd.AddArguments("--name-only", "--no-undefined").AddDynamicArguments(c.ID.String())
	data, _, err := cmd.WithDir(c.repo.Path).RunStdString(c.repo.Ctx)
	if err != nil {
		// handle special case where git can not describe commit
		if strings.Contains(err.Error(), "cannot describe") {
			return "", nil
		}

		return "", err
	}

	// name-rev commitID output will be "master" or "master~12"
	return strings.SplitN(strings.TrimSpace(data), "~", 2)[0], nil
}

// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
func GetFullCommitID(ctx context.Context, repoPath, shortID string) (string, error) {
	commitID, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(shortID).
		WithDir(repoPath).
		RunStdString(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 128") {
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
	for _, c := range s {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			return false
		}
	}
	return true
}
