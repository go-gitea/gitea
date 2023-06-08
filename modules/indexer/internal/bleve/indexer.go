// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"fmt"

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
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}
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
	if i == nil {
		return false
	}
	return i.Indexer != nil
}

func (i *Indexer) Close() {
	if i == nil {
		return
	}
	if indexer := i.Indexer; indexer != nil {
		if err := indexer.Close(); err != nil {
			log.Error("Failed to close bleve indexer in %q: %v", i.IndexDir, err)
		}
	}
}
