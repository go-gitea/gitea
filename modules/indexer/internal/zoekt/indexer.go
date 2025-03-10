// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package zoekt

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/indexer/internal/zoekt/meta"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/search"
)

type Indexer struct {
	indexDir string
	Searcher zoekt.Streamer
	version  int
}

func NewIndexer(indexDir string, version int) *Indexer {
	return &Indexer{
		indexDir: indexDir,
		version:  version,
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

	log.Info("Initializing zoekt indexer at %s", i.indexDir)
	metadata, err := meta.ReadIndexMetadata(i.indexDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err = meta.WriteIndexMetadata(i.indexDir, &meta.IndexMetadata{
				Version: i.version,
			}); err != nil {
				return false, fmt.Errorf("failed to write index metadata: %w", err)
			}
		}

		return false, err
	}
	if metadata.Version != i.version {
		compare := "older"
		if metadata.Version > i.version {
			compare = "newer"
		}

		log.Warn("Found %s zoekt index with version %d, Gitea will remove it and rebuild", compare, metadata.Version)

		// the indexer is using a previous version, so we should delete it and
		// re-populate
		if err = util.RemoveAll(i.indexDir); err != nil {
			return false, err
		}

		if _, err := os.Stat(i.indexDir); err != nil && os.IsNotExist(err) {
			exists = false
			err = os.MkdirAll(i.indexDir, 0o755)
			if err != nil {
				return false, fmt.Errorf("failed to create index directory: %w", err)
			}
		}

		if err = meta.WriteIndexMetadata(i.indexDir, &meta.IndexMetadata{
			Version: i.version,
		}); err != nil {
			return false, fmt.Errorf("failed to write index metadata: %w", err)
		}
	}

	searcher, err := search.NewDirectorySearcherFast(i.indexDir)
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
