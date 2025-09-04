// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubmoduleFile *CommitSubmoduleFile
}

func GetCommitInfoSubmoduleFile(repoLink, fullPath string, commit *Commit, refCommitID ObjectID) (*CommitSubmoduleFile, error) {
	submodule, err := commit.GetSubModule(fullPath)
	if err != nil {
		return nil, err
	}
	if submodule == nil {
		// unable to find submodule from ".gitmodules" file
		return NewCommitSubmoduleFile(repoLink, fullPath, "", refCommitID.String()), nil
	}
	return NewCommitSubmoduleFile(repoLink, fullPath, submodule.URL, refCommitID.String()), nil
}
