// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"
)

// VersionedIndexName returns the full index name with version suffix.
func (i *Indexer) VersionedIndexName() string {
	return versionedIndexName(i.indexName, i.version)
}

func versionedIndexName(indexName string, version int) string {
	if version == 0 {
		// Old index name without version
		return indexName
	}
	return fmt.Sprintf("%s.v%d", indexName, version)
}

func (i *Indexer) checkOldIndexes(ctx context.Context) {
	for v := range i.version {
		indexName := versionedIndexName(i.indexName, v)
		exists, err := i.indexExists(ctx, indexName)
		if err == nil && exists {
			log.Warn("Found older elasticsearch index named %q, Gitea will keep the old NOT DELETED. You can delete the old version after the upgrade succeed.", indexName)
		}
	}
}
