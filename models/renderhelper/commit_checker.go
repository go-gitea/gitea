// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
)

type commitChecker struct {
	ctx           context.Context
	commitCache   map[string]bool
	gitRepoFacade gitrepo.Repository

	gitRepo       *git.Repository
	gitRepoCloser io.Closer
}

func newCommitChecker(ctx context.Context, gitRepo gitrepo.Repository) *commitChecker {
	return &commitChecker{ctx: ctx, commitCache: make(map[string]bool), gitRepoFacade: gitRepo}
}

func (c *commitChecker) Close() error {
	if c != nil && c.gitRepoCloser != nil {
		return c.gitRepoCloser.Close()
	}
	return nil
}

func (c *commitChecker) IsCommitIDExisting(commitID string) bool {
	exist, inCache := c.commitCache[commitID]
	if inCache {
		return exist
	}

	if c.gitRepo == nil {
		r, closer, err := gitrepo.RepositoryFromContextOrOpen(c.ctx, c.gitRepoFacade)
		if err != nil {
			log.Error("unable to open repository: %s Error: %v", gitrepo.RepoGitURL(c.gitRepoFacade), err)
			return false
		}
		c.gitRepo, c.gitRepoCloser = r, closer
	}

	exist = c.gitRepo.IsReferenceExist(commitID) // Don't use IsObjectExist since it doesn't support short hashs with gogit edition.
	c.commitCache[commitID] = exist
	return exist
}
