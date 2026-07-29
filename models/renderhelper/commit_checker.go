// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"io"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"
)

type commitChecker struct {
	ctx          context.Context
	commitCache  map[string]bool
	repoOptional *repo_model.Repository

	gitRepo       *git.Repository
	gitRepoCloser io.Closer
}

func newCommitChecker(ctx context.Context, repo *repo_model.Repository) *commitChecker {
	return &commitChecker{ctx: ctx, commitCache: make(map[string]bool), repoOptional: repo}
}

func (c *commitChecker) Close() error {
	if c.gitRepoCloser != nil {
		return c.gitRepoCloser.Close()
	}
	return nil
}

func (c *commitChecker) IsCommitIDExisting(commitID string) bool {
	if c.repoOptional == nil {
		return false
	}
	exist, inCache := c.commitCache[commitID]
	if inCache {
		return exist
	}

	if c.gitRepo == nil {
		r, closer, err := gitrepo.RepositoryFromContextOrOpen(c.ctx, c.repoOptional)
		if err != nil {
			log.Error("unable to open repository: %s Error: %v", gitrepo.RepoGitURL(c.repoOptional), err)
			return false
		}
		c.gitRepo, c.gitRepoCloser = r, closer
	}

	exist = c.gitRepo.IsReferenceExist(commitID) // Don't use IsObjectExist since it doesn't support short hashes with gogit edition.
	c.commitCache[commitID] = exist
	return exist
}
