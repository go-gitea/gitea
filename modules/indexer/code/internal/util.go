// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"cmp"
	"slices"
	"strings"

	"gitea.dev/modules/indexer/internal"
	"gitea.dev/modules/log"
)

const filenameMatchNumberOfLines = 7 // Copied from GitHub search

func FilenameIndexerID(repoID int64, filename string) string {
	return internal.Base36(repoID) + "_" + filename
}

func ParseIndexerID(indexerID string) (int64, string) {
	before, after, ok := strings.Cut(indexerID, "_")
	if !ok {
		log.Error("Unexpected ID in repo indexer: %s", indexerID)
	}
	repoID, _ := internal.ParseBase36(before)
	return repoID, after
}

func FilenameOfIndexerID(indexerID string) string {
	_, after, ok := strings.Cut(indexerID, "_")
	if !ok {
		log.Error("Unexpected ID in repo indexer: %s", indexerID)
	}
	return after
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

// MergeMatchRanges sorts the ranges and merges the overlapping ones
func MergeMatchRanges(ranges []MatchRange) []MatchRange {
	slices.SortFunc(ranges, func(a, b MatchRange) int { return cmp.Compare(a.Start, b.Start) })
	merged := ranges[:0]
	for _, r := range ranges {
		if n := len(merged); n > 0 && r.Start < merged[n-1].End {
			merged[n-1].End = max(merged[n-1].End, r.End)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}
