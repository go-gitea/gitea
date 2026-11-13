// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"net/url"
	"path"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	code_indexer "gitea.dev/modules/indexer/code"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func ToCodeSearchResultItem(ctx context.Context, repo *repo_model.Repository, apiRepo *api.Repository, result *code_indexer.Result) *api.CodeSearchResultItem {
	fragments := code_indexer.BuildResultFragments(result.Content, result.ContentMatches)
	item := &api.CodeSearchResultItem{
		Name:        path.Base(result.Filename),
		Path:        result.Filename,
		URL:         repo.APIURL(ctx) + "/contents/" + util.PathEscapeSegments(result.Filename) + "?ref=" + url.QueryEscape(result.CommitID),
		HTMLURL:     repo.HTMLURL(ctx) + "/src/" + git.RefNameFromCommit(result.CommitID).RefWebLinkPath() + "/" + util.PathEscapeSegments(result.Filename),
		Language:    result.Language,
		Repository:  apiRepo,
		TextMatches: make([]*api.CodeSearchTextMatch, 0, len(fragments)),
	}
	for _, fragment := range fragments {
		textMatch := &api.CodeSearchTextMatch{
			ObjectURL:  item.URL,
			ObjectType: "FileContent",
			Property:   "content",
			Fragment:   fragment.Content,
			Matches:    make([]*api.CodeSearchTextMatchTerm, 0, len(fragment.Matches)),
		}
		for _, matchRange := range fragment.Matches {
			textMatch.Matches = append(textMatch.Matches, &api.CodeSearchTextMatchTerm{
				Text:    fragment.Content[matchRange.Start:matchRange.End],
				Indices: []int{matchRange.Start, matchRange.End},
			})
		}
		item.TextMatches = append(item.TextMatches, textMatch)
	}
	return item
}
