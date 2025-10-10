// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubmoduleFile *SubmoduleFile
}

func GetCommitInfoSubmoduleFile(gitRepo *Repository, repoLink, fullPath string, treeID, refCommitID ObjectID) (*SubmoduleFile, error) {
	submodule, err := NewTree(gitRepo, treeID).GetSubModule(fullPath)
	if err != nil {
		return nil, err
	}
	if submodule == nil {
		// unable to find submodule from ".gitmodules" file
		return NewSubmoduleFile(repoLink, fullPath, "", refCommitID.String()), nil
	}
	return NewSubmoduleFile(repoLink, fullPath, submodule.URL, refCommitID.String()), nil
}
