// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"context"
	"html/template"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/log"
)

type SubmoduleDiffInfo struct {
	SubmoduleName string
	SubmoduleFile *git.CommitSubmoduleFile
	NewRefID      string
	PreviousRefID string
}

func (si *SubmoduleDiffInfo) PopulateURL(diffFile *DiffFile, leftCommit, rightCommit *git.Commit) error {
	si.SubmoduleName = diffFile.Name
	submoduleCommit := rightCommit // If the submodule is added or updated, check at the right commit
	if diffFile.IsDeleted {
		submoduleCommit = leftCommit // If the submodule is deleted, check at the left commit
	}
	if submoduleCommit != nil {
		submodule, err := submoduleCommit.GetSubModule(diffFile.GetDiffFileName())
		if err != nil {
			log.Error("Unable to PopulateURL for submodule %q: GetSubModule: %v", diffFile.GetDiffFileName(), err)
			return nil // ignore the error, do not cause 500 errors for end users
		}
		if submodule != nil {
			si.SubmoduleFile = git.NewCommitSubmoduleFile(submodule.URL, submoduleCommit.ID.String())
		}
	}
	return nil
}

func (si *SubmoduleDiffInfo) PreviousRefIDLinkHTML(ctx context.Context) template.HTML {
	webLink := si.SubmoduleFile.SubmoduleWebLink(ctx, si.PreviousRefID)
	return htmlutil.HTMLFormat(`<a href="%s">%s</a>`, webLink.CommitWebLink, base.ShortSha(si.PreviousRefID))
}

func (si *SubmoduleDiffInfo) NewRefIDLinkHTML(ctx context.Context) template.HTML {
	webLink := si.SubmoduleFile.SubmoduleWebLink(ctx, si.NewRefID)
	return htmlutil.HTMLFormat(`<a href="%s">%s</a>`, webLink.CommitWebLink, base.ShortSha(si.NewRefID))
}

func (si *SubmoduleDiffInfo) CompareRefIDLinkHTML(ctx context.Context) template.HTML {
	webLink := si.SubmoduleFile.SubmoduleWebLink(ctx, si.PreviousRefID, si.NewRefID)
	return htmlutil.HTMLFormat(`<a href="%s">%s...%s</a>`, webLink.CommitWebLink, base.ShortSha(si.PreviousRefID), base.ShortSha(si.NewRefID))
}

func (si *SubmoduleDiffInfo) SubmoduleRepoLinkHTML(ctx context.Context) template.HTML {
	webLink := si.SubmoduleFile.SubmoduleWebLink(ctx)
	return htmlutil.HTMLFormat(`<a href="%s">%s</a>`, webLink.RepoWebLink, si.SubmoduleName)
}
