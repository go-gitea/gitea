// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
)

type Service interface {
	OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error)
	Run(ctx context.Context, c *git.Command, opts *git.RunOpts) error
	RepoGitURL(repo Repository) string
	WalkShowRef(ctx context.Context, repo Repository, extraArgs git.TrustedCmdArgs, skip, limit int, walkfn func(sha1, refname string) error) (countAll int, err error)
}

var _ Service = &localServiceImpl{}

type localServiceImpl struct {
	repoRootDir string
}

func (s *localServiceImpl) Run(ctx context.Context, c *git.Command, opts *git.RunOpts) error {
	opts.Dir = s.absPath(opts.Dir)
	return c.Run(opts)
}

func (s *localServiceImpl) absPath(relativePaths ...string) string {
	for _, p := range relativePaths {
		if filepath.IsAbs(p) {
			// we must panic here, otherwise there would be bugs if developers set Dir by mistake, and it would be very difficult to debug
			panic("dir field must be relative path")
		}
	}
	path := append([]string{s.repoRootDir}, relativePaths...)
	return filepath.Join(path...)
}

func (s *localServiceImpl) OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return git.OpenRepository(ctx, s.absPath(repoRelativePath(repo)))
}

func (s *localServiceImpl) RepoGitURL(repo Repository) string {
	return s.absPath(repoRelativePath(repo))
}

func (s *localServiceImpl) WalkShowRef(ctx context.Context, repo Repository, extraArgs git.TrustedCmdArgs, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	return git.WalkShowRef(ctx, s.absPath(repoRelativePath(repo)), extraArgs, skip, limit, walkfn)
}
