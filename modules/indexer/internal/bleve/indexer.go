// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"fmt"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/ethantkoenig/rupture"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic bleve indexer implementation
type Indexer struct {
	Indexer bleve.Index

	indexDir      string
	version       int
	mappingGetter MappingGetter
}

type MappingGetter func() (mapping.IndexMapping, error)

func NewIndexer(indexDir string, version int, mappingGetter func() (mapping.IndexMapping, error)) *Indexer {
	return &Indexer{
		indexDir:      indexDir,
		version:       version,
		mappingGetter: mappingGetter,
	}
}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}
	var err error
	i.Indexer, err = openIndexer(i.indexDir, i.version)
	if err != nil {
		return false, err
	}
	if i.Indexer != nil {
		return true, nil
	}

	indexMapping, err := i.mappingGetter()
	if err != nil {
		return false, err
	}

	i.Indexer, err = bleve.New(i.indexDir, indexMapping)
	if err != nil {
		return false, err
	}

	if err = rupture.WriteIndexMetadata(i.indexDir, &rupture.IndexMetadata{
		Version: i.version,
	}); err != nil {
		return false, err
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
			log.Error("Failed to close bleve indexer in %q: %v", i.indexDir, err)
		}
	}
}
