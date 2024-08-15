// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) (internal.Indexer, error) {
	version, err := inner_elasticsearch.DetectVersion(url)
	if err != nil {
		return nil, err
	}

	if version == 8 {
		return NewIndexerV8(url, indexerName), nil
	}
	return NewIndexerV7(url, indexerName), nil
}
