// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"bytes"
	"context"
	"html/template"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/timeutil"
)

// Result a search result to display
type Result struct {
	RepoID      int64
	Filename    string
	CommitID    string
	UpdatedUnix timeutil.TimeStamp
	Language    string
	Color       string
	Lines       []*ResultLine
}

type ResultLine struct {
	Num              int
	FormattedContent template.HTML
}

type SearchResultLanguages = internal.SearchResultLanguages

type SearchOptions = internal.SearchOptions

func indices(content string, selectionStartIndex, selectionEndIndex int) (int, int) {
	startIndex := selectionStartIndex
	numLinesBefore := 0
	for ; startIndex > 0; startIndex-- {
		if content[startIndex-1] == '\n' {
			if numLinesBefore == 1 {
				break
			}
			numLinesBefore++
		}
	}

	endIndex := selectionEndIndex
	numLinesAfter := 0
	for ; endIndex < len(content); endIndex++ {
		if content[endIndex] == '\n' {
			if numLinesAfter == 1 {
				break
			}
			numLinesAfter++
		}
	}

	return startIndex, endIndex
}

func writeStrings(buf *bytes.Buffer, strs ...string) error {
	for _, s := range strs {
		_, err := buf.WriteString(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func HighlightSearchResultCode(filename, language string, lineNums []int, code string) []*ResultLine {
	// we should highlight the whole code block first, otherwise it doesn't work well with multiple line highlighting
	hl, _ := highlight.Code(filename, language, code)
	highlightedLines := strings.Split(string(hl), "\n")

	// The lineNums outputted by highlight.Code might not match the original lineNums, because "highlight" removes the last `\n`
	lines := make([]*ResultLine, min(len(highlightedLines), len(lineNums)))
	for i := 0; i < len(lines); i++ {
		lines[i] = &ResultLine{
			Num:              lineNums[i],
			FormattedContent: template.HTML(highlightedLines[i]),
		}
	}
	return lines
}

func searchResult(result *internal.SearchResult, startIndex, endIndex int) (*Result, error) {
	startLineNum := 1 + strings.Count(result.Content[:startIndex], "\n")

	var formattedLinesBuffer bytes.Buffer

	contentLines := strings.SplitAfter(result.Content[startIndex:endIndex], "\n")
	lineNums := make([]int, 0, len(contentLines))
	index := startIndex
	for i, line := range contentLines {
		var err error
		if index < result.EndIndex &&
			result.StartIndex < index+len(line) &&
			result.StartIndex < result.EndIndex {
			openActiveIndex := max(result.StartIndex-index, 0)
			closeActiveIndex := min(result.EndIndex-index, len(line))
			err = writeStrings(&formattedLinesBuffer,
				line[:openActiveIndex],
				line[openActiveIndex:closeActiveIndex],
				line[closeActiveIndex:],
			)
		} else {
			err = writeStrings(&formattedLinesBuffer, line)
		}
		if err != nil {
			return nil, err
		}

		lineNums = append(lineNums, startLineNum+i)
		index += len(line)
	}

	return &Result{
		RepoID:      result.RepoID,
		Filename:    result.Filename,
		CommitID:    result.CommitID,
		UpdatedUnix: result.UpdatedUnix,
		Language:    result.Language,
		Color:       result.Color,
		Lines:       HighlightSearchResultCode(result.Filename, result.Language, lineNums, formattedLinesBuffer.String()),
	}, nil
}

// PerformSearch perform a search on a repository
func PerformSearch(ctx context.Context, opts *SearchOptions) (int, []*Result, []*SearchResultLanguages, error) {
	if opts == nil || len(opts.Keyword) == 0 {
		return 0, nil, nil, nil
	}

	total, results, resultLanguages, err := (*globalIndexer.Load()).Search(ctx, opts)
	if err != nil {
		return 0, nil, nil, err
	}

	displayResults := make([]*Result, len(results))

	for i, result := range results {
		startIndex, endIndex := indices(result.Content, result.StartIndex, result.EndIndex)
		displayResults[i], err = searchResult(result, startIndex, endIndex)
		if err != nil {
			return 0, nil, nil, err
		}
	}
	return int(total), displayResults, resultLanguages, nil
}
