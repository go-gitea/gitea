// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/process"
)

// BlameFile represents while git blame output
type BlameFile struct {
	Parts []BlamePart
}

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	Sha   string
	Lines []string
}

var blameLineRegex = regexp.MustCompile(`^([a-z0-9]*)\s*(\S*)\s*(\d*)\) (.*)`)

func parseBlameOutput(reader io.Reader) (*BlameFile, error) {

	var parts = make([]BlamePart, 0, 0)

	var blamePart *BlamePart

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		lines := blameLineRegex.FindStringSubmatch(line)

		sha1 := lines[1]
		code := lines[4]

		if blamePart == nil {
			blamePart = &BlamePart{sha1, make([]string, 0, 0)}
		}

		if blamePart.Sha != sha1 {
			parts = append(parts, *blamePart)
			blamePart = &BlamePart{sha1, make([]string, 0, 0)}
		}

		blamePart.Lines = append(blamePart.Lines, code)

	}

	if blamePart != nil {
		parts = append(parts, *blamePart)
	}

	return &BlameFile{parts}, nil
}

// GetBlame returns blame output for given repo, commit and file
func GetBlame(repoPath, commitID, file string) (*BlameFile, error) {

	_, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "blame", commitID, "-s", file)
	cmd.Dir = repoPath
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetBlame [repo_path: %s]", repoPath), cmd)
	defer process.GetManager().Remove(pid)

	blame, err := parseBlameOutput(stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return blame, nil
}
