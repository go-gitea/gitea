// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"path/filepath"
	"sync/atomic"

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

	LogString() string
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
	if setting.RepoRootPath == "" {
		panic("repo root path is not initialized")
	}
	// the repo root path and the repo loc should all have been cleaned, so we can safely join them together
	return setting.RepoRootPath + string(filepath.Separator) + filepath.FromSlash(repoLoc)
}

func repoLogNameByLocation(loc string) string {
	t := filepath.FromSlash(loc)
	// hide the parent paths, then the name should be safe for end users
	return ".../" + filepath.Base(filepath.Dir(t)) + "/" + filepath.Base(t)
}

type repositoryUnmanaged struct {
	loc     string
	logName atomic.Pointer[string]
}

func (r *repositoryUnmanaged) LogString() string {
	s := r.logName.Load()
	if s == nil {
		s = new(repoLogNameByLocation(r.loc))
		r.logName.Store(s)
	}
	return *s
}

func (r *repositoryUnmanaged) GitRepoManagedID() string {
	panic("this repo is not managed by Gitea, can't be used in this managed context")
}

func (r *repositoryUnmanaged) GitRepoLocation() string {
	return r.loc
}

func RepositoryUnmanaged(s string) RepositoryFacade {
	return &repositoryUnmanaged{loc: filepath.Clean(s)}
}

type repositoryManaged struct {
	id, loc string
	logName atomic.Pointer[string]
}

func (r *repositoryManaged) LogString() string {
	s := r.logName.Load()
	if s == nil {
		s = new(repoLogNameByLocation(r.loc))
		r.logName.Store(s)
	}
	return *s
}

func (r *repositoryManaged) GitRepoManagedID() string {
	return r.id
}

func (r *repositoryManaged) GitRepoLocation() string {
	return r.loc
}

func RepositoryManaged(id, loc string) RepositoryFacade {
	return &repositoryManaged{id: id, loc: filepath.Clean(loc)}
}
