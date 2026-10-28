// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/indexer"
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

func SearchTerms(opts *SearchOptions) []string {
	if opts.Keyword == "" {
		return nil
	}
	if opts.SearchMode == "" || opts.SearchMode == indexer.SearchModeExact {
		return []string{opts.Keyword}
	} else {
		return strings.Fields(opts.Keyword)
	}
}

// ContentMatchRanges returns the merged byte ranges of all term occurrences in content, or nil unless all terms occur
func ContentMatchRanges(content string, terms []string) []MatchRange {
	lowerContent := strings.ToLower(content)
	var lowerRanges []MatchRange
	for _, term := range terms {
		lowerTerm := strings.ToLower(term)
		found := false
		for offset := 0; ; {
			position := strings.Index(lowerContent[offset:], lowerTerm)
			if position < 0 {
				break
			}
			found = true
			offset += position + len(lowerTerm)
			lowerRanges = append(lowerRanges, MatchRange{Start: offset - len(lowerTerm), End: offset})
		}
		if !found {
			return nil
		}
	}
	lowerRanges = MergeMatchRanges(lowerRanges)
	ranges := make([]MatchRange, 0, len(lowerRanges))
	lowerOffset, index, originalStart := 0, 0, 0
	for offset := 0; index < len(lowerRanges); {
		if lowerOffset == lowerRanges[index].Start {
			originalStart = offset
		}
		if lowerOffset == lowerRanges[index].End {
			ranges = append(ranges, MatchRange{Start: originalStart, End: offset})
			index++
			continue
		}
		character, size := utf8.DecodeRuneInString(content[offset:])
		lowerOffset += utf8.RuneLen(unicode.ToLower(character))
		offset += size
	}
	return ranges
}
