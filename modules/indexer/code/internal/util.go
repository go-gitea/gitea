// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
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

// ContentMatchIndexPos returns the byte range of the earliest term in content, or -1, -1 unless all terms occur
func ContentMatchIndexPos(content string, terms []string) (int, int) {
	lowerContent := strings.ToLower(content)
	start, end := -1, -1
	for _, term := range terms {
		lowerTerm := strings.ToLower(term)
		position := strings.Index(lowerContent, lowerTerm)
		if position < 0 {
			return -1, -1
		}
		if start < 0 || position < start {
			start, end = position, position+len(lowerTerm)
		}
	}
	lowerOffset, originalStart := 0, -1
	for offset := 0; offset < len(content); {
		character, size := utf8.DecodeRuneInString(content[offset:])
		if lowerOffset == start {
			originalStart = offset
		}
		lowerOffset += utf8.RuneLen(unicode.ToLower(character))
		offset += size
		if lowerOffset == end {
			return originalStart, offset
		}
	}
	return -1, -1
}
