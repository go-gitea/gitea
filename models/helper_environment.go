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
	isWiki := "false"
	if strings.HasSuffix(repo.Name, ".wiki") {
		isWiki = "true"
	}

	sig := doer.NewGitSig()

	return append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		EnvRepoName+"="+repo.Name,
		EnvRepoUsername+"="+repo.OwnerName,
		EnvRepoIsWiki+"="+isWiki,
		EnvPusherName+"="+doer.Name,
		EnvPusherID+"="+fmt.Sprintf("%d", doer.ID),
		ProtectedBranchRepoID+"="+fmt.Sprintf("%d", repo.ID),
		"SSH_ORIGINAL_COMMAND=gitea-internal",
	)

}
