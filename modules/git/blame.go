// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"code.gitea.io/gitea/modules/process"
)

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	Sha   string
	Lines []string
}

// BlameReader returns part of file blame one by one
type BlameReader struct {
	cmd     *exec.Cmd
	pid     int64
	output  io.ReadCloser
	scanner *bufio.Scanner
	lastSha *string
}

var shaLineRegex = regexp.MustCompile("^([a-z0-9]{40})")

// NextPart returns next part of blame (sequencial code lines with the same commit)
func (r *BlameReader) NextPart() (*BlamePart, error) {
	var blamePart *BlamePart

	scanner := r.scanner

	if r.lastSha != nil {
		blamePart = &BlamePart{*r.lastSha, make([]string, 0)}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		lines := shaLineRegex.FindStringSubmatch(line)
		if lines != nil {
			sha1 := lines[1]

			if blamePart == nil {
				blamePart = &BlamePart{sha1, make([]string, 0)}
			}

			if blamePart.Sha != sha1 {
				r.lastSha = &sha1
				return blamePart, nil
			}
		} else if line[0] == '\t' {
			code := line[1:]

			blamePart.Lines = append(blamePart.Lines, code)
		}
	}

	r.lastSha = nil

	return blamePart, nil
}

// Close BlameReader - don't run NextPart after invoking that
func (r *BlameReader) Close() error {
	process.GetManager().Remove(r.pid)

	if err := r.cmd.Wait(); err != nil {
		return fmt.Errorf("Wait: %v", err)
	}

	return nil
}

// CreateBlameReader creates reader for given repository, commit and file
func CreateBlameReader(repoPath, commitID, file string) (*BlameReader, error) {
	gitRepo, err := OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	gitRepo.Close()

	return createBlameReader(repoPath, GitExecutable, "blame", commitID, "--porcelain", "--", file)
}

func createBlameReader(dir string, command ...string) (*BlameReader, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetBlame [repo_path: %s]", dir), cmd)

	scanner := bufio.NewScanner(stdout)

	return &BlameReader{
		cmd,
		pid,
		stdout,
		scanner,
		nil,
	}, nil
}
