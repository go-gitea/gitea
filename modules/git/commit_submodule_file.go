// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"path"
	"strings"

	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/util"
)

// CommitSubmoduleFile represents a file with submodule type.
type CommitSubmoduleFile struct {
	repoLink string
	fullPath string
	refURL   string
	refID    string

	parsed           bool
	parsedTargetLink string
}

// NewCommitSubmoduleFile create a new submodule file
func NewCommitSubmoduleFile(repoLink, fullPath, refURL, refID string) *CommitSubmoduleFile {
	return &CommitSubmoduleFile{repoLink: repoLink, fullPath: fullPath, refURL: refURL, refID: refID}
}

// RefID returns the commit ID of the submodule, it returns empty string for nil receiver
func (sf *CommitSubmoduleFile) RefID() string {
	if sf == nil {
		return ""
	}
	return sf.refID
}

func (sf *CommitSubmoduleFile) getWebLinkInTargetRepo(ctx context.Context, moreLinkPath string) *SubmoduleWebLink {
	if sf == nil || sf.refURL == "" {
		return nil
	}
	if strings.HasPrefix(sf.refURL, "../") {
		targetLink := path.Join(sf.repoLink, path.Dir(sf.fullPath), sf.refURL)
		return &SubmoduleWebLink{RepoWebLink: targetLink, CommitWebLink: targetLink + moreLinkPath}
	}
	if !sf.parsed {
		sf.parsed = true
		parsedURL, err := giturl.ParseRepositoryURL(ctx, sf.refURL)
		if err != nil {
			return nil
		}
		sf.parsedTargetLink = giturl.MakeRepositoryWebLink(parsedURL)
	}
	return &SubmoduleWebLink{RepoWebLink: sf.parsedTargetLink, CommitWebLink: sf.parsedTargetLink + moreLinkPath}
}

// SubmoduleWebLinkTree tries to make the submodule's tree link in its own repo, it also works on "nil" receiver
// It returns nil if the submodule does not have a valid URL or is nil
func (sf *CommitSubmoduleFile) SubmoduleWebLinkTree(ctx context.Context, optCommitID ...string) *SubmoduleWebLink {
	return sf.getWebLinkInTargetRepo(ctx, "/tree/"+util.OptionalArg(optCommitID, sf.RefID()))
}

// SubmoduleWebLinkCompare tries to make the submodule's compare link in its own repo, it also works on "nil" receiver
// It returns nil if the submodule does not have a valid URL or is nil
func (sf *CommitSubmoduleFile) SubmoduleWebLinkCompare(ctx context.Context, commitID1, commitID2 string) *SubmoduleWebLink {
	return sf.getWebLinkInTargetRepo(ctx, "/compare/"+commitID1+"..."+commitID2)
}
