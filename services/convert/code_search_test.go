// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	code_indexer "gitea.dev/modules/indexer/code"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestToCodeSearchResultItem(t *testing.T) {
	repo := &repo_model.Repository{OwnerName: "owner", Name: "repo"}
	result := &code_indexer.Result{
		Filename:       "dir/file.go",
		CommitID:       "0123456789012345678901234567890123456789",
		Language:       "Go",
		Content:        "héllo wörld\n\n\n\nsecond fragment",
		ContentMatches: []code_indexer.MatchRange{{Start: 0, End: 6}, {Start: 7, End: 13}, {Start: 17, End: 23}},
	}
	apiRepo := &api.Repository{ID: 1}
	fileURL := repo.APIURL(t.Context()) + "/contents/dir/file.go?ref=" + result.CommitID
	assert.Equal(t, &api.CodeSearchResultItem{
		Name:       "file.go",
		Path:       "dir/file.go",
		URL:        fileURL,
		HTMLURL:    repo.HTMLURL(t.Context()) + "/src/commit/" + result.CommitID + "/dir/file.go",
		Language:   "Go",
		Repository: apiRepo,
		TextMatches: []*api.CodeSearchTextMatch{
			{
				ObjectURL:  fileURL,
				ObjectType: "FileContent",
				Property:   "content",
				Fragment:   "héllo wörld\n\n",
				Matches: []*api.CodeSearchTextMatchTerm{
					{Text: "héllo", Indices: []int{0, 6}},
					{Text: "wörld", Indices: []int{7, 13}},
				},
			},
			{
				ObjectURL:  fileURL,
				ObjectType: "FileContent",
				Property:   "content",
				Fragment:   "\nsecond fragment",
				Matches:    []*api.CodeSearchTextMatchTerm{{Text: "second", Indices: []int{1, 7}}},
			},
		},
	}, ToCodeSearchResultItem(t.Context(), repo, apiRepo, result))

	result.ContentMatches = nil
	assert.Equal(t, []*api.CodeSearchTextMatch{}, ToCodeSearchResultItem(t.Context(), repo, nil, result).TextMatches)
}
