// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"

	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface to use database's like search
type Indexer struct {
	indexer_internal.Indexer
}

func NewIndexer() *Indexer {
	return &Indexer{
		Indexer: &inner_db.Indexer{},
	}
}

// Index dummy function
func (i *Indexer) Index(_ context.Context, _ ...*internal.IndexerData) error {
	return nil
}

// Delete dummy function
func (i *Indexer) Delete(_ context.Context, _ ...int64) error {
	return nil
}

var SearchFunc func(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error)

// Search searches for issues
func (i *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	if SearchFunc != nil {
		return SearchFunc(ctx, options)
	}
	return nil, fmt.Errorf("SearchFunc is not registered")
}
