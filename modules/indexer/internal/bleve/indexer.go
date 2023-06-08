// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"

	"github.com/blevesearch/bleve/v2"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic bleve indexer implementation
type Indexer struct {
	IndexDir string
	Indexer  bleve.Index
	Version  int
}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	var err error
	i.Indexer, err = openIndexer(i.IndexDir, i.Version)
	if err != nil {
		return false, err
	}
	if i.Indexer != nil {
		return true, nil
	}
	return false, nil
}

// Ping checks if the indexer is available
func (i *Indexer) Ping() bool {
	return i.Indexer != nil
}

func (i *Indexer) Close() {
	if indexer := i.Indexer; indexer != nil {
		if err := indexer.Close(); err != nil {
			log.Error("Failed to close bleve indexer in %q: %v", i.IndexDir, err)
		}
	}
}
