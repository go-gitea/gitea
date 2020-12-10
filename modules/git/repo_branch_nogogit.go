// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"io"
	"strings"
)

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	return IsReferenceExist(repo.Path, BranchPrefix+name)
}

// GetBranches returns all branches of the repository.
func (repo *Repository) GetBranches() ([]string, error) {
	return callShowRef(repo.Path, BranchPrefix, "--heads")
}

func callShowRef(repoPath, prefix, arg string) ([]string, error) {
	var branchNames []string

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderrBuilder := &strings.Builder{}
		err := NewCommand("show-ref", arg).RunInDirPipeline(repoPath, stdoutWriter, stderrBuilder)
		if err != nil {
			if stderrBuilder.Len() == 0 {
				_ = stdoutWriter.Close()
				return
			}
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderrBuilder.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	bufReader := bufio.NewReader(stdoutReader)
	for {
		// The output of show-ref is simply a list:
		// <sha> SP <ref> LF
		_, err := bufReader.ReadSlice(' ')
		for err == bufio.ErrBufferFull {
			// This shouldn't happen but we'll tolerate it for the sake of peace
			_, err = bufReader.ReadSlice(' ')
		}
		if err == io.EOF {
			return branchNames, nil
		}
		if err != nil {
			return nil, err
		}

		branchName, err := bufReader.ReadString('\n')
		if err == io.EOF {
			// This shouldn't happen... but we'll tolerate it for the sake of peace
			return branchNames, nil
		}
		if err != nil {
			return nil, err
		}
		branchName = strings.TrimPrefix(branchName, prefix)
		if len(branchName) > 0 {
			branchName = branchName[:len(branchName)-1]
		}
		branchNames = append(branchNames, branchName)
	}
}
