// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"html/template"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/log"
)

type SubmoduleInfo struct {
	SubmoduleURL  string
	NewRefID      string
	PreviousRefID string
}

func (si *SubmoduleInfo) PopulateURL(diffFile *DiffFile, leftCommit, rightCommit *git.Commit) error {
	var submoduleCommit *git.Commit
	switch {
	case diffFile.IsDeleted:
		submoduleCommit = leftCommit // If the submodule is removed, we need to check it at the left commit
	default:
		submoduleCommit = rightCommit // If the submodule path is added or updated, we check this at the right commit
	}
	if submoduleCommit != nil {
		submodule, err := submoduleCommit.GetSubModule(diffFile.GetDiffFileName())
		if err != nil {
			log.Error("Unable to PopulateURL for submodule %q: GetSubModule: %v", diffFile.GetDiffFileName(), err)
			return nil // ignore the error, do not cause 500 errors for end users
		}
		if submodule != nil {
			si.SubmoduleURL = submodule.URL
		}
	}
	return nil
}

func (si *SubmoduleInfo) NewRefIDLinkHTML() template.HTML {
	refURL := si.refURL()
	if si.PreviousRefID == "" {
		return htmlutil.HTMLFormat(`<a href="%s/commit/%s"">%s</a>`, refURL, si.NewRefID, base.ShortSha(si.NewRefID))
	}
	return htmlutil.HTMLFormat(`<a href="%s/compare/%s...%s">%s</a>`, refURL, si.PreviousRefID, si.NewRefID, base.ShortSha(si.NewRefID))
}

func (si *SubmoduleInfo) PreviousRefIDLinkHTML() template.HTML {
	refURL := si.refURL()
	return htmlutil.HTMLFormat(`<a href="%s/commit/%s">%s</a>`, refURL, si.PreviousRefID, base.ShortSha(si.PreviousRefID))
}

func (si *SubmoduleInfo) SubmoduleRepoLinkHTML(name string) template.HTML {
	refURL := si.refURL()
	return htmlutil.HTMLFormat(`<a href="%s">%s</a>`, refURL, name)
}

// RefURL guesses and returns reference URL.
func (si *SubmoduleInfo) refURL() string {
	// FIXME: use unified way to handle domain and subpath
	// FIXME: RefURL(repoFullName) is only used to provide relative path support for submodules
	// it is not our use case because it won't work for git clone via http/ssh
	// FIXME: the current RefURL is not right, it doesn't consider the subpath
	return git.NewCommitSubModuleFile(si.SubmoduleURL, "").RefURL("")
}
