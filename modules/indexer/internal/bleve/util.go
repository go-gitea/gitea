// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"errors"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/ethantkoenig/rupture"
)

// openIndexer open the index at the specified path, checking for metadata
// updates and bleve version updates.  If index needs to be created (or
// re-created), returns (nil, nil)
func openIndexer(path string, latestVersion int) (bleve.Index, int, error) {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return nil, 0, nil
	} else if err != nil {
		return nil, 0, err
	}

	metadata, err := rupture.ReadIndexMetadata(path)
	if err != nil {
		return nil, 0, err
	}
	if metadata.Version < latestVersion {
		// the indexer is using a previous version, so we should delete it and
		// re-populate
		return nil, metadata.Version, util.RemoveAll(path)
	}

	index, err := bleve.Open(path)
	if err != nil {
		if errors.Is(err, upsidedown.IncompatibleVersion) {
			log.Warn("Indexer was built with a previous version of bleve, deleting and rebuilding")
			return nil, 0, util.RemoveAll(path)
		}
		return nil, 0, err
	}

	return index, 0, nil
}
