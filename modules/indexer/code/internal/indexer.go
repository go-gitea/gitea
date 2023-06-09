// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/indexer/internal"
)

// Indexer defines an interface to index and search code contents
type Indexer interface {
	internal.Indexer
	Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *RepoChanges) error
	Delete(repoID int64) error
	Search(ctx context.Context, repoIDs []int64, language, keyword string, page, pageSize int, isMatch bool) (int64, []*SearchResult, []*SearchResultLanguages, error)
}
