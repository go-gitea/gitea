// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git/service"
)

// AddChanges marks local changes to be ready for commit.
func AddChanges(repoPath string, all bool, files ...string) error {
	return AddChangesWithArgs(repoPath, GlobalCommandArgs, all, files...)
}

// AddChangesWithArgs marks local changes to be ready for commit.
func AddChangesWithArgs(repoPath string, gloablArgs []string, all bool, files ...string) error {
	cmd := NewCommandNoGlobals(append(gloablArgs, "add")...)
	if all {
		cmd.AddArguments("--all")
	}
	cmd.AddArguments("--")
	_, err := cmd.AddArguments(files...).RunInDir(repoPath)
	return err
}

// CommitChangesOptions the options when a commit created
type CommitChangesOptions struct {
	Committer *service.Signature
	Author    *service.Signature
	Message   string
}

// CommitChanges commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChanges(repoPath string, opts CommitChangesOptions) error {
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return CommitChangesWithArgs(repoPath, cargs, opts)
}

// CommitChangesWithArgs commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChangesWithArgs(repoPath string, args []string, opts CommitChangesOptions) error {
	cmd := NewCommandNoGlobals(args...)
	if opts.Committer != nil {
		cmd.AddArguments("-c", "user.name="+opts.Committer.Name, "-c", "user.email="+opts.Committer.Email)
	}
	cmd.AddArguments("commit")

	if opts.Author == nil {
		opts.Author = opts.Committer
	}
	if opts.Author != nil {
		cmd.AddArguments(fmt.Sprintf("--author='%s <%s>'", opts.Author.Name, opts.Author.Email))
	}
	cmd.AddArguments("-m", opts.Message)

	_, err := cmd.RunInDir(repoPath)
	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

// AllCommitsCount returns count of all commits in repository
func AllCommitsCount(repoPath string, hidePRRefs bool, files ...string) (int64, error) {
	args := []string{"--all", "--count"}
	if hidePRRefs {
		args = append([]string{"--exclude=refs/pull/*"}, args...)
	}
	cmd := NewCommand("rev-list")
	cmd.AddArguments(args...)
	if len(files) > 0 {
		cmd.AddArguments("--")
		cmd.AddArguments(files...)
	}

	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCountFiles returns number of total commits of until given revision.
func CommitsCountFiles(repoPath string, revision, relpath []string) (int64, error) {
	cmd := NewCommand("rev-list", "--count")
	cmd.AddArguments(revision...)
	if len(relpath) > 0 {
		cmd.AddArguments("--")
		cmd.AddArguments(relpath...)
	}

	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(repoPath string, revision ...string) (int64, error) {
	return CommitsCountFiles(repoPath, revision, []string{})
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

	fields := strings.Fields(searchString)
	for _, k := range fields {
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

// GetCommitFileStatus returns file status of commit in given repository.
func GetCommitFileStatus(repoPath, commitID string) (*CommitFileStatus, error) {
	stdout, w := io.Pipe()
	done := make(chan struct{})
	fileStatus := NewCommitFileStatus()
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}

			switch fields[0][0] {
			case 'A':
				fileStatus.Added = append(fileStatus.Added, fields[1])
			case 'D':
				fileStatus.Removed = append(fileStatus.Removed, fields[1])
			case 'M':
				fileStatus.Modified = append(fileStatus.Modified, fields[1])
			}
		}
		done <- struct{}{}
	}()

	stderr := new(bytes.Buffer)
	err := NewCommand("show", "--name-status", "--pretty=format:''", commitID).RunInDirPipeline(repoPath, w, stderr)
	w.Close() // Close writer to exit parsing goroutine
	if err != nil {
		return nil, ConcatenateError(err, stderr.String())
	}

	<-done
	return fileStatus, nil
}
