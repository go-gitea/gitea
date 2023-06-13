// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
)

// VersionedIndexName returns the full index name with version
func (i *Indexer) VersionedIndexName() string {
	return versionedIndexName(i.indexName, i.version)
}

func versionedIndexName(indexName string, version int) string {
	return fmt.Sprintf("%s.v%d", indexName, version)
}

func (i *Indexer) checkOldIndexes() {
	i.checkOldIndex(i.indexName) // Old index name without version
	for v := 1; v < i.version; v++ {
		i.checkOldIndex(versionedIndexName(i.indexName, v))
	}
}

func (i *Indexer) checkOldIndex(indexName string) {
	_, err := i.Client.GetIndex(indexName)
	if err == nil {
		log.Warn("Found older meilisearch index named %q, Gitea will keep the old NOT DELETED. You can delete the old version after the upgrade succeed.", indexName)
	}
}
