// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"strings"

	"gitea.dev/modules/util"
)

func RepoCodeGitRepoRelativePath(ownerName, repoName string) string {
	return util.PathJoinRelX(strings.ToLower(ownerName), strings.ToLower(repoName)+".git")
}

func RepoWikiGitRepoRelativePath(ownerName, repoName string) string {
	return util.PathJoinRelX(strings.ToLower(ownerName), strings.ToLower(repoName)+".wiki.git")
}

// CodeRepoByName returns an unmanaged repository facade for the code repository of the given owner and repository name.
// Usually it is used for migration fixes or repository adoption/creation/rename/transfer.
func CodeRepoByName(ownerName, repoName string) RepositoryFacade {
	return RepositoryUnmanaged(RepoCodeGitRepoRelativePath(ownerName, repoName))
}

func WikiRepoByName(ownerName, repoName string) RepositoryFacade {
	return RepositoryUnmanaged(RepoWikiGitRepoRelativePath(ownerName, repoName))
}
