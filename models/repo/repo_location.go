// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strconv"

	"gitea.dev/modules/git/gitrepo"
)

func repoCodeGitRepoManagedID(repoID int64) string {
	return "repo-" + strconv.FormatInt(repoID, 10)
}

func (repo *Repository) CodeStorageRepo() gitrepo.RepositoryFacade {
	id := repoCodeGitRepoManagedID(repo.ID)
	relPath := gitrepo.RepoCodeGitRepoRelativePath(repo.OwnerName, repo.Name)
	return gitrepo.RepositoryManaged(id, relPath)
}

func (repo *Repository) GitRepoLocation() string {
	// TODO: use CodeGitRepo instead of this one
	return gitrepo.RepoCodeGitRepoRelativePath(repo.OwnerName, repo.Name)
}

func (repo *Repository) GitRepoManagedID() string {
	// TODO: use CodeGitRepo instead of this one
	return repoCodeGitRepoManagedID(repo.ID)
}

func (repo *Repository) WikiStorageRepo() gitrepo.RepositoryFacade {
	// The wiki repository should have the same object format as the code repository. TODO: old comment, REALLY? Why?
	id := "repo-wiki-" + strconv.FormatInt(repo.ID, 10)
	repoPath := gitrepo.RepoWikiGitRepoRelativePath(repo.OwnerName, repo.Name)
	return gitrepo.RepositoryManaged(id, repoPath)
}
