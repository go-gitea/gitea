// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

func RepoGitURL(repo Repository) string {
	return curService.GitURL(repoRelativePath(repo))
}

func WikiRepoGitURL(repo Repository) string {
	return curService.GitURL(wikiRelativePath(repo))
}

func GetRepoOrWikiGitURL(repo Repository, isWiki bool) string {
	if isWiki {
		return WikiRepoGitURL(repo)
	}
	return RepoGitURL(repo)
}
