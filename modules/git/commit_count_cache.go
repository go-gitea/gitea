// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/modules/cache"
)

func makeCommitsCountCacheKey(repo RepositoryFacade, ref RefName) string {
	return cache.SafeCacheKey("git-commits-count:"+repo.GitRepoManagedID(), ref.String())
}

func RemoveCommitsCountCache(repo RepositoryFacade, ref RefName) {
	cache.Remove(makeCommitsCountCacheKey(repo, ref))
}

func GetCommitsCountCache(ctx context.Context, repo RepositoryFacade, ref RefName, commit *Commit) (int64, error) {
	if commit == nil {
		return 0, nil
	}
	return cache.GetInt64(makeCommitsCountCacheKey(repo, ref), func() (int64, error) {
		return CommitsCountOfCommit(ctx, repo, commit.ID.String())
	})
}
