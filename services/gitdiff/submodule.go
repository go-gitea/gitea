// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import "code.gitea.io/gitea/modules/git"

type SubmoduleInfo struct {
	SubmoduleURL  string
	NewRefID      string
	PreviousRefID string
}

func (si *SubmoduleInfo) PopulateURL(diffFile *DiffFile, leftCommit, rightCommit *git.Commit) error {
	// If the submodule is removed, we need to check it at the left commit
	if diffFile.IsDeleted {
		if leftCommit == nil {
			return nil
		}

		submodule, err := leftCommit.GetSubModule(diffFile.GetDiffFileName())
		if err != nil {
			return err
		}

		if submodule != nil {
			si.SubmoduleURL = submodule.URL
		}

		return nil
	}

	// Even if the submodule path is updated, we check this at the right commit
	submodule, err := rightCommit.GetSubModule(diffFile.Name)
	if err != nil {
		return err
	}

	if submodule != nil {
		si.SubmoduleURL = submodule.URL
	}
	return nil
}

func (si *SubmoduleInfo) RefID() string {
	if si.NewRefID != "" {
		return si.NewRefID
	}
	return si.PreviousRefID
}

// RefURL guesses and returns reference URL.
func (si *SubmoduleInfo) RefURL(repoFullName string) string {
	return git.NewCommitSubModuleFile(si.SubmoduleURL, si.RefID()).RefURL(repoFullName)
}
