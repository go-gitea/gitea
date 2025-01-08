// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	giturl "code.gitea.io/gitea/modules/git/url"
)

// CommitSubmoduleFile represents a file with submodule type.
type CommitSubmoduleFile struct {
	refURL    string
	parsedURL *giturl.RepositoryURL
	parsed    bool
	refID     string
	repoLink  string
}

// NewCommitSubmoduleFile create a new submodule file
func NewCommitSubmoduleFile(refURL, refID string) *CommitSubmoduleFile {
	return &CommitSubmoduleFile{refURL: refURL, refID: refID}
}

func (sf *CommitSubmoduleFile) RefID() string {
	return sf.refID // this function is only used in templates
}

// SubmoduleWebLink tries to make some web links for a submodule, it also works on "nil" receiver
func (sf *CommitSubmoduleFile) SubmoduleWebLink(ctx context.Context, optCommitID ...string) *SubmoduleWebLink {
	if sf == nil {
		return nil
	}
	if !sf.parsed {
		sf.parsed = true
		parsedURL, err := giturl.ParseRepositoryURL(ctx, sf.refURL)
		if err != nil {
			return nil
		}
		sf.parsedURL = parsedURL
		sf.repoLink = giturl.MakeRepositoryWebLink(sf.parsedURL)
	}
	var commitLink string
	if len(optCommitID) == 2 {
		commitLink = sf.repoLink + "/compare/" + optCommitID[0] + "..." + optCommitID[1]
	} else if len(optCommitID) == 1 {
		commitLink = sf.repoLink + "/commit/" + optCommitID[0]
	} else {
		commitLink = sf.repoLink + "/commit/" + sf.refID
	}
	return &SubmoduleWebLink{RepoWebLink: sf.repoLink, CommitWebLink: commitLink}
}
