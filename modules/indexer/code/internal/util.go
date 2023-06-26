// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"strings"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"
)

func FilenameIndexerID(repoID int64, filename string) string {
	return internal.Base36(repoID) + "_" + filename
}

func ParseIndexerID(indexerID string) (int64, string) {
	index := strings.IndexByte(indexerID, '_')
	if index == -1 {
		log.Error("Unexpected ID in repo indexer: %s", indexerID)
	}
	repoID, _ := internal.ParseBase36(indexerID[:index])
	return repoID, indexerID[index+1:]
}

func FilenameOfIndexerID(indexerID string) string {
	index := strings.IndexByte(indexerID, '_')
	if index == -1 {
		log.Error("Unexpected ID in repo indexer: %s", indexerID)
	}
	return indexerID[index+1:]
}
