// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"context"

	"code.gitea.io/gitea/modules/indexer/internal"
)

// Indexer defines an interface to indexer issues contents
type Indexer interface {
	internal.Indexer
	Index(issue []*IndexerData) error
	Delete(ids ...int64) error
	Search(ctx context.Context, kw string, repoIDs []int64, limit, start int) (*SearchResult, error)
}
