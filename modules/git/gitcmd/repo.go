// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"path/filepath"

	"gitea.dev/modules/setting"
)

type RepositoryFacade interface {
	GitRepoUniqueID() string
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
