// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

/*
GitHub, GitLab, Gogs: *.wiki.git
BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// WikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func WikiRemoteURL(ctx context.Context, remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	for _, suffix := range commonWikiURLSuffixes {
		wikiURL := remote + suffix
		if git.IsRepoURLAccessible(ctx, wikiURL) {
			return wikiURL
		}
	}
	return ""
}

// OpenWikiRepository opens the repository at the given relative path with the provided context.
func OpenWikiRepository(ctx context.Context, repoPath *repo_model.Repository) (*Repository, error) {
	return git.OpenRepository(ctx, repoPath.WikiPath())
}
