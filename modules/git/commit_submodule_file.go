// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	giturl "code.gitea.io/gitea/modules/git/url"
)

// CommitSubmoduleFile represents a file with submodule type.
type CommitSubmoduleFile struct {
	refURL string
	refID  string

	parsed         bool
	targetRepoLink string
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
		if strings.HasPrefix(sf.refURL, "../") {
			// FIXME: when handling relative path, this logic is not right. It needs to:
			// 1. Remember the submodule's full path and its commit's repo home link
			// 2. Resolve the relative path: targetRepoLink = path.Join(repoHomeLink, path.Dir(submoduleFullPath), refURL)
			// Not an easy task and need to refactor related code a lot.
			sf.targetRepoLink = sf.refURL
		} else {
			parsedURL, err := giturl.ParseRepositoryURL(ctx, sf.refURL)
			if err != nil {
				return nil
			}
			sf.targetRepoLink = giturl.MakeRepositoryWebLink(parsedURL)
		}
	}
	var commitLink string
	if len(optCommitID) == 2 {
		commitLink = sf.targetRepoLink + "/compare/" + optCommitID[0] + "..." + optCommitID[1]
	} else if len(optCommitID) == 1 {
		commitLink = sf.targetRepoLink + "/tree/" + optCommitID[0]
	} else {
		commitLink = sf.targetRepoLink + "/tree/" + sf.refID
	}
	return &SubmoduleWebLink{RepoWebLink: sf.targetRepoLink, CommitWebLink: commitLink}
}
