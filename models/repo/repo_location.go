// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

func repoCodeGitRepoRelativePath(ownerName, repoName string) string {
	return strings.ToLower(ownerName) + "/" + strings.ToLower(repoName) + ".git"
}

func repoWikiGitRepoRelativePath(ownerName, repoName string) string {
	return strings.ToLower(ownerName) + "/" + strings.ToLower(repoName) + ".wiki.git"
}

// CodeRepoByName returns an unmanaged repository facade for the code repository of the given owner and repository name.
// Usually it is used for migration fixes or repository adoption/creation/rename/transfer.
func CodeRepoByName(ownerName, repoName string) gitcmd.RepositoryFacade {
	return gitcmd.RepositoryUnmanaged(repoCodeGitRepoRelativePath(ownerName, repoName))
}

func WikiRepoByName(ownerName, repoName string) gitcmd.RepositoryFacade {
	return gitcmd.RepositoryUnmanaged(repoWikiGitRepoRelativePath(ownerName, repoName))
}

func repoCodeGitRepoManagedID(repoID int64) string {
	return "repo-" + strconv.FormatInt(repoID, 10)
}

func (repo *Repository) CodeStorageRepo() gitcmd.RepositoryFacade {
	id := repoCodeGitRepoManagedID(repo.ID)
	repoPath := repoCodeGitRepoRelativePath(repo.OwnerName, repo.Name)
	return gitcmd.RepositoryManaged(id, repoPath)
}

func (repo *Repository) GitRepoLocation() string {
	// TODO: use CodeGitRepo instead of this one
	return repoCodeGitRepoRelativePath(repo.OwnerName, repo.Name)
}

func (repo *Repository) GitRepoManagedID() string {
	// TODO: use CodeGitRepo instead of this one
	return repoCodeGitRepoManagedID(repo.ID)
}

func (repo *Repository) WikiStorageRepo() gitcmd.RepositoryFacade {
	// The wiki repository should have the same object format as the code repository. TODO: old comment, REALLY? Why?
	id := "repo-wiki-" + strconv.FormatInt(repo.ID, 10)
	repoPath := repoWikiGitRepoRelativePath(repo.OwnerName, repo.Name)
	return gitcmd.RepositoryManaged(id, repoPath)
}
