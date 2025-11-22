// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package zoekt

import (
	"context"
	"fmt"
	"os"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/search"
)

type Indexer struct {
	indexDir string
	Searcher zoekt.Streamer
}

func NewIndexer(indexDir string) *Indexer {
	return &Indexer{
		indexDir: indexDir,
	}
}

func (i *Indexer) Init(_ context.Context) (bool, error) {
	exists := true

	if _, err := os.Stat(i.indexDir); err != nil && os.IsNotExist(err) {
		exists = false
		err = os.MkdirAll(i.indexDir, 0o755)
		if err != nil {
			return false, fmt.Errorf("failed to create index directory: %w", err)
		}
	}

	// TODO: change to use shards.NewDirectorySearcherFast
	searcher, err := search.NewDirectorySearcher(i.indexDir)
	if err != nil {
		return false, err
	}
	i.Searcher = searcher

	return exists, nil
}

func (i *Indexer) Ping(_ context.Context) error {
	// NOTHING TO DO
	return nil
}

func (i *Indexer) Close() {
	if i.Searcher == nil {
		return
	}
	i.Searcher.Close()
}
