// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"path/filepath"

	"gitea.dev/modules/setting"
)

type RepositoryFacade interface {
	// GitRepoManagedID returns a "managed id", which should be a cache-key-friendly string.
	// e.g.: ID with prefix&suffix or UUID
	GitRepoManagedID() string

	// GitRepoLocation returns the location for the git repository.
	// * relative path: will be converted to an absolute path by setting.RepoRootPath
	// * absolute path: will be used as-is
	// * in the future: maybe URI for more flexible definitions
	GitRepoLocation() string
}

func (c *Command) WithRepo(repo RepositoryFacade) *Command {
	c.opts.Dir = RepoLocalPath(repo)
	return c
}

func RepoLocalPath(repo RepositoryFacade) string {
	repoLoc := repo.GitRepoLocation()
	if filepath.IsAbs(repoLoc) {
		return repoLoc
	}
	return filepath.Join(setting.RepoRootPath, filepath.FromSlash(repoLoc))
}

type repositoryUnmanaged string

func (r repositoryUnmanaged) GitRepoManagedID() string {
	panic("this repo is not managed by Gitea, can't be used in this managed context")
}

func (r repositoryUnmanaged) GitRepoLocation() string {
	return string(r)
}

func RepositoryUnmanaged(s string) RepositoryFacade {
	return repositoryUnmanaged(s)
}

type repositoryManaged struct {
	id  string
	loc string
}

func (r *repositoryManaged) GitRepoManagedID() string {
	return r.id
}

func (r *repositoryManaged) GitRepoLocation() string {
	return r.loc
}

func RepositoryManaged(id, loc string) RepositoryFacade {
	return &repositoryManaged{id, loc}
}
