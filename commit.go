// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mcuadros/go-version"
)

// Commit represents a git commit.
type Commit struct {
	Tree
	ID            sha1 // The ID of this commit object
	Author        *Signature
	Committer     *Signature
	CommitMessage string

	parents []sha1 // SHA1 strings
	// submodules map[string]*SubModule
}

func (c *Commit) GetCommitOfRelPath(relpath string) (*Commit, error) {
	return c.repo.getCommitOfRelPath(c.ID, relpath)
}

// AddAllChanges marks local changes to be ready for commit.
func AddChanges(repoPath string, all bool, files ...string) error {
	cmd := NewCommand("add")
	if all {
		cmd.AddArguments("--all")
	}
	_, err := cmd.AddArguments(files...).RunInDir(repoPath)
	return err
}

// CommitChanges commits local changes with given message and author.
func CommitChanges(repoPath, message string, author *Signature) error {
	cmd := NewCommand("commit", "-m", message)
	if author != nil {
		cmd.AddArguments(fmt.Sprintf("--author='%s <%s>'", author.Name, author.Email))
	}
	_, err := cmd.RunInDir(repoPath)

	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(repoPath, revision string) (int64, error) {
	if version.Compare(gitVersion, "1.8.0", "<") {
		stdout, err := NewCommand("log", "--pretty=format:''", revision).RunInDir(repoPath)
		if err != nil {
			return 0, err
		}
		return int64(len(strings.Split(stdout, "\n"))), nil
	}

	stdout, err := NewCommand("rev-list", "--count", revision).RunInDir(repoPath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}
