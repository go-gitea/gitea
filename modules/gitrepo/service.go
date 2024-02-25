// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

type Service interface {
	Run(Repository, *git.Command, *RunOpts) error
}

var _ Service = &localServiceImpl{}

type localServiceImpl struct {
	repoRootDir string
}

func (s *localServiceImpl) repoPath(repo Repository) string {
	return filepath.Join(s.repoRootDir, strings.ToLower(repo.GetOwnerName()), strings.ToLower(repo.GetName())+".git")
}

func (s *localServiceImpl) wikiPath(repo Repository) string {
	return filepath.Join(s.repoRootDir, strings.ToLower(repo.GetOwnerName()), strings.ToLower(repo.GetName())+".wiki.git")
}

func (s *localServiceImpl) Run(repo Repository, c *git.Command, opts *RunOpts) error {
	if opts.Dir != "" {
		// we must panic here, otherwise there would be bugs if developers set Dir by mistake, and it would be very difficult to debug
		panic("dir field must be empty when using RunStdBytes")
	}

	if opts.IsWiki {
		opts.Dir = s.wikiPath(repo)
	} else {
		opts.Dir = s.repoPath(repo)
	}
	return c.Run(&opts.RunOpts)
}
