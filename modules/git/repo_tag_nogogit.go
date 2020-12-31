// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"io"
	"strings"
)

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	return IsReferenceExist(repo.Path, TagPrefix+name)
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	return callTag(repo.Path, "--list")
}

func callTag(repoPath, arg string) ([]string, error) {
	var tagNames []string

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderrBuilder := &strings.Builder{}
		err := NewCommand("tag", arg).RunInDirPipeline(repoPath, stdoutWriter, stderrBuilder)
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
		// The output of tag is simply a list:
		// LF
		tagName, err := bufReader.ReadString('\n')
		if err == io.EOF {
			// Reverse order
			for i := 0; i < len(tagNames)/2; i++ {
				j := len(tagNames) - i - 1
				tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
			}

			return tagNames, nil
		}
		if err != nil {
			// This shouldn't happen... but we'll tolerate it for the sake of peace
			return nil, err
		}
		tagName = strings.TrimSpace(tagName)
		tagNames = append(tagNames, tagName)
	}
}
