// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"io"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

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
func GetCommitFileStatus(ctx context.Context, repo Repository, commitID string) (*CommitFileStatus, error) {
	stdout, w := io.Pipe()
	done := make(chan struct{})
	fileStatus := NewCommitFileStatus()
	go func() {
		parseCommitFileStatus(fileStatus, stdout)
		close(done)
	}()

	stderr := new(bytes.Buffer)
	err := gitcmd.NewCommand("log", "--name-status", "-m", "--pretty=format:", "--first-parent", "--no-renames", "-z", "-1").
		AddDynamicArguments(commitID).
		WithDir(repoPath(repo)).
		WithStdout(w).
		WithStderr(stderr).
		Run(ctx)
	w.Close() // Close writer to exit parsing goroutine
	if err != nil {
		return nil, gitcmd.ConcatenateError(err, stderr.String())
	}

	<-done
	return fileStatus, nil
}
