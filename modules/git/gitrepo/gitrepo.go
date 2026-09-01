// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// RepoLocalPath returns an absolute path for a RepositoryFacade.
// TODO: most of the calls to this function should be replaced with a "Repo FS" in the future
// to handle file accesses in the git repo (e.g.: read, write, list, remove).
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

func UserLocalPath(userName string) string {
	if setting.RepoRootPath == "" {
		panic("repo root path is not initialized")
	}
	return filepath.Join(setting.RepoRootPath, filepath.Clean(strings.ToLower(userName)))
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

// RepositoryUnmanaged returns a RepositoryFacade for a repository that might not be managed by Gitea.
// If the path is not absolute, then it is relative to setting.RepoRootPath
// This function is mainly for maintaining the owner's repo when the repo is not managed yet.
// e.g.: init, clone, transfer, rename, adopt, etc., and temp repo creation and modification.
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

func RepoLocalFS(repo RepositoryFacade) fs.FS {
	return os.DirFS(RepoLocalPath(repo))
}
