// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import repo_model "code.gitea.io/gitea/models/repo"

func RepoGitURL(repo *repo_model.Repository) string {
	return repo.RepoPath()
}

func WikiRepoGitURL(repo *repo_model.Repository) string {
	return repo.WikiPath()
}
