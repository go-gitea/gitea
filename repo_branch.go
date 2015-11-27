// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"
)

const BRANCH_PREFIX = "refs/heads/"

// Branch represents a Git branch.
type Branch struct {
	Name string
	Path string
}

// GetHEADBranch returns corresponding branch of HEAD.
func (repo *Repository) GetHEADBranch() (*Branch, error) {
	stdout, err := NewCommand("symbolic-ref", "HEAD").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	stdout = strings.TrimSpace(stdout)

	if !strings.HasPrefix(stdout, BRANCH_PREFIX) {
		return nil, fmt.Errorf("invalid HEAD branch: %v", stdout)
	}

	return &Branch{
		Name: stdout[len(BRANCH_PREFIX):],
		Path: stdout,
	}, nil
}

// IsBranchExist returns true if given branch exists in repository.
func IsBranchExist(repoPath, branch string) bool {
	_, err := NewCommand("show-ref", "--verify", BRANCH_PREFIX+branch).RunInDir(repoPath)
	return err == nil
}
