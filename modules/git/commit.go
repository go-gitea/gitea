// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// Commit represents a git commit.
type Commit struct {
	ID            ObjectID
	TreeID        ObjectID
	Parents       []ObjectID // ID strings
	Author        *Signature // never nil
	Committer     *Signature // never nil
	CommitMessage string
	Signature     *CommitSignature
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

// ParentCount returns number of parents of the commit.
// 0 if this is the root commit,  otherwise 1,2, etc.
func (c *Commit) ParentCount() int {
	return len(c.Parents)
}

// AddChanges marks local changes to be ready for commit.
func AddChanges(ctx context.Context, repoPath string, all bool, files ...string) error {
	cmd := gitcmd.NewCommand().AddArguments("add")
	if all {
		cmd.AddArguments("--all")
	}
	cmd.AddDashesAndList(files...)
	_, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
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

	_, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

// AllCommitsCount returns count of all commits in repository
func AllCommitsCount(ctx context.Context, repoPath string, hidePRRefs bool, files ...string) (int64, error) {
	cmd := gitcmd.NewCommand("rev-list")
	if hidePRRefs {
		cmd.AddArguments("--exclude=" + PullPrefix + "*")
	}
	cmd.AddArguments("--all", "--count")
	if len(files) > 0 {
		cmd.AddDashesAndList(files...)
	}

	stdout, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCountOptions the options when counting commits
type CommitsCountOptions struct {
	RepoPath string
	Not      string
	Revision []string
	RelPath  []string
	Since    string
	Until    string
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(ctx context.Context, opts CommitsCountOptions) (int64, error) {
	cmd := gitcmd.NewCommand("rev-list", "--count")

	cmd.AddDynamicArguments(opts.Revision...)

	if opts.Not != "" {
		cmd.AddOptionValues("--not", opts.Not)
	}

	if len(opts.RelPath) > 0 {
		cmd.AddDashesAndList(opts.RelPath...)
	}

	stdout, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: opts.RepoPath})
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
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

// CommitFileStatus represents status of files in a commit.
type CommitFileStatus struct {
	Added    []string
	Removed  []string
	Modified []string
}

// NewCommitFileStatus creates a CommitFileStatus
func NewCommitFileStatus() *CommitFileStatus {
	return &CommitFileStatus{
		[]string{}, []string{}, []string{},
	}
}

func parseCommitFileStatus(fileStatus *CommitFileStatus, stdout io.Reader) {
	rd := bufio.NewReader(stdout)
	peek, err := rd.Peek(1)
	if err != nil {
		if err != io.EOF {
			log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
		}
		return
	}
	if peek[0] == '\n' || peek[0] == '\x00' {
		_, _ = rd.Discard(1)
	}
	for {
		modifier, err := rd.ReadString('\x00')
		if err != nil {
			if err != io.EOF {
				log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
			}
			return
		}
		file, err := rd.ReadString('\x00')
		if err != nil {
			if err != io.EOF {
				log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
			}
			return
		}
		file = file[:len(file)-1]
		switch modifier[0] {
		case 'A':
			fileStatus.Added = append(fileStatus.Added, file)
		case 'D':
			fileStatus.Removed = append(fileStatus.Removed, file)
		case 'M':
			fileStatus.Modified = append(fileStatus.Modified, file)
		}
	}
}

// GetCommitFileStatus returns file status of commit in given repository.
func GetCommitFileStatus(ctx context.Context, repoPath, commitID string) (*CommitFileStatus, error) {
	stdout, w := io.Pipe()
	done := make(chan struct{})
	fileStatus := NewCommitFileStatus()
	go func() {
		parseCommitFileStatus(fileStatus, stdout)
		close(done)
	}()

	stderr := new(bytes.Buffer)
	err := gitcmd.NewCommand("log", "--name-status", "-m", "--pretty=format:", "--first-parent", "--no-renames", "-z", "-1").AddDynamicArguments(commitID).Run(ctx, &gitcmd.RunOpts{
		Dir:    repoPath,
		Stdout: w,
		Stderr: stderr,
	})
	w.Close() // Close writer to exit parsing goroutine
	if err != nil {
		return nil, gitcmd.ConcatenateError(err, stderr.String())
	}

	<-done
	return fileStatus, nil
}

// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
func GetFullCommitID(ctx context.Context, repoPath, shortID string) (string, error) {
	commitID, _, err := gitcmd.NewCommand("rev-parse").AddDynamicArguments(shortID).RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
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
