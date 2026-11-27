// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"html"
	"html/template"
	"strings"
	"unicode/utf8"

	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/util"
)

type Result = internal.SearchResult

const (
	MaxResultFragments    = 10
	MaxResultFragmentSize = 4 * 1024
)

type ResultFragment struct {
	StartLine int
	Content   string       // UTF-8 clipped at a rune boundary
	Matches   []MatchRange // byte ranges into Content
}

type ResultLine struct {
	Num              int
	FormattedContent template.HTML
}

type SearchResultLanguages = internal.SearchResultLanguages

type MatchRange = internal.MatchRange

type SearchOptions = internal.SearchOptions

// RenderSearchResultCode returns the escaped lines of the code with the matches wrapped in <mark>, the frontend highlights them
func RenderSearchResultCode(lineNums []int, code string, matches ...MatchRange) []*ResultLine {
	lines := make([]*ResultLine, 0, len(lineNums))
	offset := 0
	for line := range strings.Lines(code) {
		if len(lines) == len(lineNums) {
			break
		}
		var sb strings.Builder
		pos, lineEnd := 0, offset+len(line)
		for len(matches) > 0 && matches[0].Start < lineEnd {
			start, end := max(matches[0].Start-offset, pos), min(matches[0].End-offset, len(line))
			if start < end {
				sb.WriteString(html.EscapeString(line[pos:start]))
				sb.WriteString("<mark>" + html.EscapeString(line[start:end]) + "</mark>")
				pos = end
			}
			if matches[0].End > lineEnd {
				break // it continues on the next line
			}
			matches = matches[1:]
		}
		sb.WriteString(html.EscapeString(line[pos:]))
		lines = append(lines, &ResultLine{Num: lineNums[len(lines)], FormattedContent: template.HTML(sb.String())})
		offset = lineEnd
	}
	return lines
}

// PerformSearch perform a search on a repository
func PerformSearch(ctx context.Context, opts *SearchOptions) (int64, []*Result, []*SearchResultLanguages, error) {
	if opts == nil || len(opts.Terms) == 0 {
		return 0, nil, nil, nil
	}
	return (*globalIndexer.Load()).Search(ctx, opts)
}

func BuildResultFragments(content string, matches []MatchRange) []*ResultFragment {
	if len(matches) == 0 {
		return nil
	}
	var blocks []MatchRange
	for _, match := range matches {
		lineStart := strings.LastIndexByte(content[:match.Start], '\n') + 1
		start := strings.LastIndexByte(content[:max(0, lineStart-1)], '\n') + 1
		_, matchRuneSize := utf8.DecodeRuneInString(content[match.Start:match.End])
		if match.Start+matchRuneSize-start > MaxResultFragmentSize {
			start = lineStart
			if match.Start+matchRuneSize-start > MaxResultFragmentSize {
				start = match.Start
			}
		}
		end := match.End - 1
		limit := min(len(content), end+MaxResultFragmentSize+utf8.UTFMax) // fragments are truncated to MaxResultFragmentSize anyway
		for range 2 {
			newline := strings.IndexByte(content[end:limit], '\n')
			if newline < 0 {
				end = limit
				break
			}
			end += newline + 1
		}
		if len(blocks) > 0 && start <= blocks[len(blocks)-1].End {
			previous := &blocks[len(blocks)-1]
			mergedEnd := previous.Start + len(util.TruncateStringBytes(content[previous.Start:max(previous.End, end)], MaxResultFragmentSize))
			if match.Start+matchRuneSize <= mergedEnd {
				previous.End = mergedEnd
				continue
			}
			start = previous.End
		}
		if len(blocks) == MaxResultFragments {
			break
		}
		blocks = append(blocks, MatchRange{Start: start, End: start + len(util.TruncateStringBytes(content[start:end], MaxResultFragmentSize))})
	}
	var fragments []*ResultFragment
	for _, block := range blocks {
		fragment := &ResultFragment{StartLine: 1 + strings.Count(content[:block.Start], "\n"), Content: content[block.Start:block.End]}
		for _, match := range matches {
			if match.Start < block.End && match.End > block.Start {
				fragment.Matches = append(fragment.Matches, MatchRange{
					Start: max(match.Start, block.Start) - block.Start,
					End:   min(match.End, block.End) - block.Start,
				})
			}
		}
		fragments = append(fragments, fragment)
	}
	return fragments
}
