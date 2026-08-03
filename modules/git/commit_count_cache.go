// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"gitea.dev/modules/cache"
)

func makeCommitsCountCacheKey(repo RepositoryFacade, ref RefName) string {
	return fmt.Sprintf("git-commits-count:%s:%s", repo.GitRepoManagedID(), cache.SafeCacheKey(ref.String(), 200))
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
