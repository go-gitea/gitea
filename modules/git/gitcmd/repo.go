// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"path/filepath"

	"gitea.dev/modules/setting"
)

type RepositoryFacade interface {
	GitRepoUniqueID() string
	RelativePath() string
}

func (c *Command) WithRepo(repo RepositoryFacade) *Command {
	c.opts.Dir = filepath.Join(setting.RepoRootPath, filepath.FromSlash(repo.RelativePath()))
	return c
}
