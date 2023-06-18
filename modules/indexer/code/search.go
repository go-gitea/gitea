// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"bytes"
	"context"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// Result a search result to display
type Result struct {
	RepoID         int64
	Filename       string
	CommitID       string
	UpdatedUnix    timeutil.TimeStamp
	Language       string
	Color          string
	LineNumbers    []int
	FormattedLines string
}

type SearchResultLanguages = internal.SearchResultLanguages

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

func searchResult(result *internal.SearchResult, startIndex, endIndex int) (*Result, error) {
	startLineNum := 1 + strings.Count(result.Content[:startIndex], "\n")

	var formattedLinesBuffer bytes.Buffer

	contentLines := strings.SplitAfter(result.Content[startIndex:endIndex], "\n")
	lineNumbers := make([]int, len(contentLines))
	index := startIndex
	for i, line := range contentLines {
		var err error
		if index < result.EndIndex &&
			result.StartIndex < index+len(line) &&
			result.StartIndex < result.EndIndex {
			openActiveIndex := util.Max(result.StartIndex-index, 0)
			closeActiveIndex := util.Min(result.EndIndex-index, len(line))
			err = writeStrings(&formattedLinesBuffer,
				line[:openActiveIndex],
				line[openActiveIndex:closeActiveIndex],
				line[closeActiveIndex:],
			)
		} else {
			err = writeStrings(&formattedLinesBuffer,
				line,
			)
		}
		if err != nil {
			return nil, err
		}

		lineNumbers[i] = startLineNum + i
		index += len(line)
	}

	highlighted, _ := highlight.Code(result.Filename, "", formattedLinesBuffer.String())

	return &Result{
		RepoID:         result.RepoID,
		Filename:       result.Filename,
		CommitID:       result.CommitID,
		UpdatedUnix:    result.UpdatedUnix,
		Language:       result.Language,
		Color:          result.Color,
		LineNumbers:    lineNumbers,
		FormattedLines: highlighted,
	}, nil
}

// PerformSearch perform a search on a repository
func PerformSearch(ctx context.Context, repoIDs []int64, language, keyword string, page, pageSize int, isMatch bool) (int, []*Result, []*internal.SearchResultLanguages, error) {
	if len(keyword) == 0 {
		return 0, nil, nil, nil
	}

	total, results, resultLanguages, err := (*globalIndexer.Load()).Search(ctx, repoIDs, language, keyword, page, pageSize, isMatch)
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
