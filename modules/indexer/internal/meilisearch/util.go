// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

// IndexName returns the full index name with version
func (i *Indexer) IndexName() string {
	return i.indexerName
}
