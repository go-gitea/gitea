// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"io"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
)

type commitChecker struct {
	ctx          context.Context
	commitCache  map[string]string
	repoOptional *repo_model.Repository

	gitRepo       *git.Repository
	gitRepoCloser io.Closer
}

func newCommitChecker(ctx context.Context, repo *repo_model.Repository) *commitChecker {
	return &commitChecker{ctx: ctx, commitCache: make(map[string]string), repoOptional: repo}
}

func (c *commitChecker) Close() error {
	if c.gitRepoCloser != nil {
		return c.gitRepoCloser.Close()
	}
	return nil
}

func (c *commitChecker) ResolveCommitID(commitID string) string {
	if c.repoOptional == nil {
		return ""
	}
	fullID, inCache := c.commitCache[commitID]
	if inCache {
		return fullID
	}

	if c.gitRepo == nil {
		r, closer, err := git.RepositoryFromContextOrOpen(c.ctx, c.repoOptional)
		if err != nil {
			log.Error("Unable to open repository: %s, error: %v", c.repoOptional.FullName(), err)
			return ""
		}
		c.gitRepo, c.gitRepoCloser = r, closer
	}

	if commit, err := c.gitRepo.GetCommit(c.ctx, commitID); err == nil && strings.HasPrefix(commit.ID.String(), commitID) { // GetCommit resolves refs and peels tags too
		fullID = commit.ID.String()
	}
	c.commitCache[commitID] = fullID
	return fullID
}
