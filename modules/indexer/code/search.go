// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"html"
	"html/template"
	"strings"
	"unicode/utf8"

	"gitea.dev/modules/highlight"
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

func HighlightSearchResultCode(filename, language string, lineNums []int, code string, matches ...MatchRange) []*ResultLine {
	lexer := highlight.DetectChromaLexerByFileName(filename, language)
	hl := markMatches(code, string(highlight.RenderCodeByLexer(lexer, code)), matches)
	highlightedLines := highlight.UnsafeSplitHighlightedLines(template.HTML(hl))

	lines := make([]*ResultLine, min(len(highlightedLines), len(lineNums)))
	for i := range lines {
		lines[i] = &ResultLine{
			Num:              lineNums[i],
			FormattedContent: template.HTML(highlightedLines[i]),
		}
	}
	return lines
}

// markMatches wraps the code byte ranges in <mark>, split at tags, skipping code bytes the lexer dropped
func markMatches(code, highlighted string, matches []MatchRange) string {
	if len(matches) == 0 {
		return highlighted
	}
	var marked strings.Builder
	marked.Grow(len(highlighted))
	offset, open := 0, false
	for highlighted != "" {
		var token string
		switch highlighted[0] {
		case '<':
			token = highlighted[:strings.IndexByte(highlighted, '>')+1]
			highlighted = highlighted[len(token):]
			if open {
				marked.WriteString("</mark>")
				open = false
			}
			marked.WriteString(token)
			continue
		case '&':
			token = highlighted[:strings.IndexByte(highlighted, ';')+1]
		default:
			token = highlighted[:1]
		}
		highlighted = highlighted[len(token):]
		text := html.UnescapeString(token)
		for offset < len(code) && !strings.HasPrefix(code[offset:], text) {
			offset++
		}
		for len(matches) > 0 && matches[0].End <= offset {
			matches = matches[1:]
		}
		if inMatch := len(matches) > 0 && matches[0].Start <= offset; inMatch != open {
			marked.WriteString(util.Iif(inMatch, "<mark>", "</mark>"))
			open = inMatch
		}
		marked.WriteString(token)
		offset += len(text)
	}
	if open {
		marked.WriteString("</mark>")
	}
	return marked.String()
}

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
		for range 2 {
			newline := strings.IndexByte(content[end:], '\n')
			if newline < 0 {
				end = len(content)
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
