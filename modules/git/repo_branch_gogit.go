// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package git

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	reference, err := repo.gogitRepo.Reference(plumbing.ReferenceName(BranchPrefix+name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// GetBranches returns all branches of the repository.
func (repo *Repository) GetBranches() ([]string, error) {
	var branchNames []string

	branches, err := repo.gogitRepo.Branches()
	if err != nil {
		return nil, err
	}

	_ = branches.ForEach(func(branch *plumbing.Reference) error {
		branchNames = append(branchNames, strings.TrimPrefix(branch.Name().String(), BranchPrefix))
		return nil
	})

	// TODO: Sort?

	return branchNames, nil
}
