// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"strings"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"
)

const filenameMatchNumberOfLines = 7 // Copied from GitHub search

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

// FilenameMatchIndexPos returns the boundaries of its first seven lines.
func FilenameMatchIndexPos(content string) (int, int) {
	count := 1
	for i, c := range content {
		if c == '\n' {
			count++
			if count == filenameMatchNumberOfLines {
				return 0, i
			}
		}
	}
	return 0, len(content)
}
