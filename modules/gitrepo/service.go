// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type Service interface {
	Run(cmd *git.Command, opts *git.RunOpts) error
	OpenRepository(ctx context.Context, relativePath string) (GitRepository, error)
	IsRepositoryExist(ctx context.Context, relativePath string) (bool, error)
	RenameDir(oldRelativePath, newRelativePath string) error
	RemoveDir(relativePath string) error
	GitURL(relativePath string) string
	ForkRepository(ctx context.Context, baseRelativePath, targetRelativePath, singleBranch string) error
	CheckDelegateHooks(ctx context.Context, relativePath string) ([]string, error)
	CreateDelegateHooks(ctx context.Context, relativePath string) (err error)
	WalkReferences(ctx context.Context, relativePath string, walkfn func(sha1, refname string) error) (int, error)
}

var _ Service = &localServiceImpl{}

type localServiceImpl struct {
	repoRootDir string
}

func (s *localServiceImpl) Run(c *git.Command, opts *git.RunOpts) error {
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

func (s *localServiceImpl) OpenRepository(ctx context.Context, relativePath string) (GitRepository, error) {
	return git.OpenRepository(ctx, s.absPath(relativePath))
}

func (s *localServiceImpl) IsRepositoryExist(ctx context.Context, relativePath string) (bool, error) {
	return util.IsExist(s.absPath(relativePath))
}

func (s *localServiceImpl) RenameDir(oldRelativePath, newRelativePath string) error {
	return util.Rename(s.absPath(oldRelativePath), s.absPath(newRelativePath))
}

func (s *localServiceImpl) RemoveDir(relativePath string) error {
	return util.RemoveAll(s.absPath(relativePath))
}

func (s *localServiceImpl) GitURL(relativePath string) string {
	return s.absPath(relativePath)
}

func (s *localServiceImpl) ForkRepository(ctx context.Context, baseRelativePath, targetRelativePath, singleBranch string) error {
	cloneCmd := git.NewCommand(ctx, "clone", "--bare")
	if singleBranch != "" {
		cloneCmd.AddArguments("--single-branch", "--branch").AddDynamicArguments(singleBranch)
	}

	if stdout, _, err := cloneCmd.AddDynamicArguments(s.absPath(baseRelativePath), s.absPath(targetRelativePath)).
		SetDescription(fmt.Sprintf("ForkRepository(git clone): %s to %s", s.GitURL(baseRelativePath), s.GitURL(targetRelativePath))).
		RunStdBytes(&git.RunOpts{Timeout: 10 * time.Minute}); err != nil {
		log.Error("Fork Repository (git clone) Failed for %v (from %v):\nStdout: %s\nError: %v", targetRelativePath, baseRelativePath, stdout, err)
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func (s *localServiceImpl) CheckDelegateHooks(ctx context.Context, relativePath string) ([]string, error) {
	return checkDelegateHooks(ctx, s.absPath(relativePath))
}

func (s *localServiceImpl) CreateDelegateHooks(ctx context.Context, relativePath string) (err error) {
	return createDelegateHooks(ctx, s.absPath(relativePath))
}

func (s *localServiceImpl) WalkReferences(ctx context.Context, relativePath string, walkfn func(sha1, refname string) error) (int, error) {
	return git.WalkShowRef(ctx, s.absPath(relativePath), nil, 0, 0, walkfn)
}
