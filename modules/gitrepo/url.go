// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

func RepoGitURL(repo Repository) string {
	return repoPath(repo)
}
