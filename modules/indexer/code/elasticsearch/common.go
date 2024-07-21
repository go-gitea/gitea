// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"code.gitea.io/gitea/modules/indexer/code/internal"
)

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string, version int) internal.Indexer {
	if version == 8 {
		return NewIndexerV8(url, indexerName)
	}
	return NewIndexerV7(url, indexerName)
}
