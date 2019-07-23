// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"strings"
)

// PushingEnvironment returns an os environment to allow hooks to work on push
func PushingEnvironment(doer *User, repo *Repository) []string {
	return FullPushingEnvironment(doer, doer, repo.MustOwnerName(), repo.Name, repo.ID, 0)
}

// FullPushingEnvironment returns an os environment to allow hooks to work on push
func FullPushingEnvironment(author, committer *User, ownerName, repoName string, repoID, prID int64) []string {
	isWiki := "false"
	if strings.HasSuffix(repoName, ".wiki") {
		isWiki = "true"
	}

	authorSig := author.NewGitSig()
	committerSig := committer.NewGitSig()

	// We should add "SSH_ORIGINAL_COMMAND=gitea-internal",
	// once we have hook and pushing infrastructure working correctly
	return append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_COMMITTER_NAME="+committerSig.Name,
		"GIT_COMMITTER_EMAIL="+committerSig.Email,
		EnvRepoName+"="+repoName,
		EnvRepoUsername+"="+ownerName,
		EnvRepoIsWiki+"="+isWiki,
		EnvPusherName+"="+committer.Name,
		EnvPusherID+"="+fmt.Sprintf("%d", committer.ID),
		ProtectedBranchRepoID+"="+fmt.Sprintf("%d", repoID),
		ProtectedBranchPRID+"="+fmt.Sprintf("%d", prID),
		"SSH_ORIGINAL_COMMAND=gitea-internal",
	)

}
