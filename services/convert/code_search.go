// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"unicode/utf8"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	code_indexer "gitea.dev/modules/indexer/code"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// ToCodeSearchResultItem converts a code search result in the repository to an API item
func ToCodeSearchResultItem(ctx context.Context, repo *repo_model.Repository, apiRepo *api.Repository, result *code_indexer.Result) *api.CodeSearchResultItem {
	fileURL := repo.APIURL(ctx) + "/contents/" + util.PathEscapeSegments(result.Filename) + "?ref=" + url.QueryEscape(result.CommitID)
	item := &api.CodeSearchResultItem{
		Name:        path.Base(result.Filename),
		Path:        result.Filename,
		URL:         fileURL,
		HTMLURL:     repo.HTMLURL(ctx) + "/src/" + git.RefNameFromCommit(result.CommitID).RefWebLinkPath() + "/" + util.PathEscapeSegments(result.Filename),
		Repository:  apiRepo,
		LineNumbers: make([]string, len(result.Lines)),
		TextMatches: []*api.CodeSearchTextMatch{toCodeSearchTextMatch(fileURL, result.RawContent, result.ContentMatches)},
	}
	for i, line := range result.Lines {
		item.LineNumbers[i] = strconv.Itoa(line.Num)
	}
	return item
}

func toCodeSearchTextMatch(objectURL, fragment string, ranges []code_indexer.MatchRange) *api.CodeSearchTextMatch {
	textMatch := &api.CodeSearchTextMatch{
		ObjectURL:  objectURL,
		ObjectType: "FileContent",
		Property:   "content",
		Fragment:   fragment,
		Matches:    make([]*api.CodeSearchTextMatchTerm, 0, len(ranges)),
	}
	for _, r := range ranges {
		textMatch.Matches = append(textMatch.Matches, &api.CodeSearchTextMatchTerm{
			Text:    fragment[r.Start:r.End],
			Indices: []int{utf8.RuneCountInString(fragment[:r.Start]), utf8.RuneCountInString(fragment[:r.End])}, // characters, not bytes
		})
	}
	return textMatch
}
