// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"cmp"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/analyze"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/internal"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"golang.org/x/text/unicode/norm"
)

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

type FileMatcher []struct {
	text    string
	pattern *regexp.Regexp
}

func NewFileMatcher(terms []indexer.SearchTerm) (FileMatcher, error) {
	matcher := make(FileMatcher, len(terms))
	for i, term := range terms {
		matcher[i].text = strings.ToLower(term.Text)
		if term.Kind == indexer.SearchTermRegexp {
			pattern, err := regexp.Compile("(?i)" + term.Text)
			if err != nil {
				return nil, &indexer.SearchQueryError{Message: err.Error()}
			}
			matcher[i].pattern = pattern
		}
	}
	return matcher, nil
}

func LiteralMatcher(terms []string) FileMatcher {
	matcher := make(FileMatcher, len(terms))
	for i, term := range terms {
		matcher[i].text = strings.ToLower(term)
	}
	return matcher
}

func (m FileMatcher) MatchFile(path, content string) (ranges []MatchRange, ok bool) {
	var lowerPath, lowerContent string
	lowered := false
	ok = len(m) > 0
	var lowerRanges []MatchRange
	for _, term := range m {
		found := false
		if pattern := term.pattern; pattern != nil {
			offset, count := 0, 0
			for line := range strings.Lines(content) {
				line = strings.TrimSuffix(line, "\n")
				for _, location := range pattern.FindAllStringIndex(line, 1000-count) {
					found = true
					count++
					if location[0] < location[1] {
						ranges = append(ranges, MatchRange{Start: offset + location[0], End: offset + location[1]})
					}
				}
				offset += len(line) + 1
				if count == 1000 {
					break
				}
			}
			found = found || pattern.MatchString(norm.NFC.String(path))
		} else if term.text != "" {
			if !lowered {
				lowerPath, lowerContent = norm.NFC.String(strings.ToLower(path)), strings.ToLower(content)
				lowered = true
			}
			for offset, count := 0, 0; count < 1000; count++ {
				position := strings.Index(lowerContent[offset:], term.text)
				if position < 0 {
					break
				}
				found = true
				offset += position + len(term.text)
				lowerRanges = append(lowerRanges, MatchRange{Start: offset - len(term.text), End: offset})
			}
			found = found || strings.Contains(lowerPath, norm.NFC.String(term.text))
		}
		if !found {
			return nil, false
		}
	}
	lowerRanges = MergeMatchRanges(lowerRanges)
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
	return MergeMatchRanges(ranges), ok
}

func IsSearchable(path string, size int64) bool {
	if size > setting.Indexer.MaxIndexerFileSize || setting.Indexer.ExcludeVendored && analyze.IsVendor(path) {
		return false
	}
	name := strings.ToLower(path)
	for _, pattern := range setting.Indexer.ExcludePatterns {
		if pattern.Match(name) {
			return false
		}
	}
	for _, pattern := range setting.Indexer.IncludePatterns {
		if pattern.Match(name) {
			return true
		}
	}
	return len(setting.Indexer.IncludePatterns) == 0
}

func ReadFile(batch git.CatFileBatch, update FileUpdate) ([]byte, bool, error) {
	if !IsSearchable(update.Filename, update.Size) {
		return nil, false, nil
	}
	info, reader, err := batch.QueryContent(update.BlobSha)
	if err != nil {
		return nil, false, err
	}
	content, err := io.ReadAll(io.LimitReader(reader, info.Size))
	if err != nil {
		return nil, false, err
	}
	if !typesniffer.DetectContentType(content).IsText() {
		content = nil
	}
	_, err = reader.Discard(1)
	return charset.ToUTF8DropErrors(content), true, err
}

func (m FileMatcher) MatchResult(result *SearchResult) bool {
	var ok bool
	result.ContentMatches, ok = m.MatchFile(result.Filename, result.Content)
	result.Color = enry.GetColor(result.Language)
	return ok
}
