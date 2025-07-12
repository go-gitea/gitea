// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "path"

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubmoduleFile *CommitSubmoduleFile
}

func getCommitInfoSubmoduleFile(repoLink string, entry *TreeEntry, commit *Commit, treePathDir string) (*CommitSubmoduleFile, error) {
	fullPath := path.Join(treePathDir, entry.Name())
	submodule, err := commit.GetSubModule(fullPath)
	if err != nil {
		return nil, err
	}
	return NewCommitSubmoduleFile(repoLink, fullPath, submodule.URL, entry.ID.String()), nil
}
