// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(repoPath, name string) bool {
	_, err := NewCommand("show-ref", "--verify", "--", name).RunInDir(repoPath)
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(repoPath, name string) bool {
	return IsReferenceExist(repoPath, BranchPrefix+name)
}
