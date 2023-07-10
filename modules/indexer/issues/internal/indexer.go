// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/indexer/internal"
)

// Indexer defines an interface to indexer issues contents
type Indexer interface {
	internal.Indexer
	Index(ctx context.Context, issue []*IndexerData) error
	Delete(ctx context.Context, ids ...int64) error
	Search(ctx context.Context, kw string, repoIDs []int64, limit, start int, state string) (*SearchResult, error)
}

// NewDummyIndexer returns a dummy indexer
func NewDummyIndexer() Indexer {
	return &dummyIndexer{
		Indexer: internal.NewDummyIndexer(),
	}
}

type dummyIndexer struct {
	internal.Indexer
}

func (d *dummyIndexer) Index(ctx context.Context, issue []*IndexerData) error {
	return fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Delete(ctx context.Context, ids ...int64) error {
	return fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Search(ctx context.Context, kw string, repoIDs []int64, limit, start int, state string) (*SearchResult, error) {
	return nil, fmt.Errorf("indexer is not ready")
}
