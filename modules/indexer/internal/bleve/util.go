// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"os"

	"code.gitea.io/gitea/modules/util"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/ethantkoenig/rupture"
)

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
