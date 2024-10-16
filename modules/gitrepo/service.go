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
	WalkReferences(ctx context.Context, repo Repository, refType git.ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error)
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

func (s *localServiceImpl) WalkReferences(ctx context.Context, repo Repository, refType git.ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	gitRepo, err := s.OpenRepository(ctx, repo)
	if err != nil {
		return 0, err
	}
	defer gitRepo.Close()
	return gitRepo.WalkReferences(refType, skip, limit, walkfn)
}
