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
	if version == 0 {
		// Old index name without version
		return indexName
	}

	// The format of the index name is <index_name>_v<version>, not <index_name>.v<version> like elasticsearch.
	// Because meilisearch does not support "." in index name, it should contain only alphanumeric characters, hyphens (-) and underscores (_).
	// See https://www.meilisearch.com/docs/learn/core_concepts/indexes#index-uid

	return fmt.Sprintf("%s_v%d", indexName, version)
}

func (i *Indexer) checkOldIndexes() {
	for v := 0; v < i.version; v++ {
		indexName := versionedIndexName(i.indexName, v)
		_, err := i.Client.GetIndex(indexName)
		if err == nil {
			log.Warn("Found older meilisearch index named %q, Gitea will keep the old NOT DELETED. You can delete the old version after the upgrade succeed.", indexName)
		}
	}
}
