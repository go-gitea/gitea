// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
)

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// UnstableGuessRefByShortName does the best guess to see whether a "short name" provided by user is a branch, tag or commit.
// It could guess wrongly if the input is already ambiguous. For example:
// * "refs/heads/the-name" vs "refs/heads/refs/heads/the-name"
// * "refs/tags/1234567890" vs commit "1234567890"
// In most cases, it SHOULD AVOID using this function, unless there is an irresistible reason (eg: make API friendly to end users)
// If the function is used, the caller SHOULD CHECK the ref type carefully.
func (repo *Repository) UnstableGuessRefByShortName(shortName string) RefName {
	if repo.IsBranchExist(shortName) {
		return RefNameFromBranch(shortName)
	}
	if repo.IsTagExist(shortName) {
		return RefNameFromTag(shortName)
	}
	if strings.HasPrefix(shortName, "refs/") {
		if repo.IsReferenceExist(shortName) {
			return RefName(shortName)
		}
	}
	commit, err := repo.GetCommit(shortName)
	if err == nil {
		commitIDString := commit.ID.String()
		if strings.HasPrefix(commitIDString, shortName) {
			return RefName(commitIDString)
		}
	}
	return ""
}
