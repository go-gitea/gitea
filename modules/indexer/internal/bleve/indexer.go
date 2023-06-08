// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"os"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/ethantkoenig/rupture"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a bleve indexer implementation
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

// openIndexer open the index at the specified path, checking for metadata
// updates and bleve version updates.  If index needs to be created (or
// re-created), returns (nil, nil)
func openIndexer(path string, latestVersion int) (bleve.Index, error) {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	metadata, err := rupture.ReadIndexMetadata(path)
	if err != nil {
		return nil, err
	}
	if metadata.Version < latestVersion {
		// the indexer is using a previous version, so we should delete it and
		// re-populate
		return nil, util.RemoveAll(path)
	}

	index, err := bleve.Open(path)
	if err != nil && err == upsidedown.IncompatibleVersion {
		// the indexer was built with a previous version of bleve, so we should
		// delete it and re-populate
		return nil, util.RemoveAll(path)
	} else if err != nil {
		return nil, err
	}

	return index, nil
}
