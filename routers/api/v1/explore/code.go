// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// Code explore code
func Code(ctx *context.APIContext) {
	// swagger:operation GET /explore/code explore codeSearch
	// ---
	// summary: Search for code
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: fuzzy
	//   in: query
	//   description: whether to search fuzzy or strict
	//   type: boolean
	// responses:
	//   "200":
	//     description: "SearchResults of a successful search"
	//     schema:
	//			 "$ref": "#/definitions/ExploreCodeResult"
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.NotFound("Indexer not enabled")
		return
	}

	keyword := ctx.FormTrim("q")

	isFuzzy := ctx.FormOptionalBool("fuzzy").ValueOrDefault(true)

	if keyword == "" {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: "No keyword provided",
		})
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repoIDs []int64
		err     error
		isAdmin bool
	)
	if ctx.Doer != nil {
		isAdmin = ctx.Doer.IsAdmin
	}

	// guest user or non-admin user
	if ctx.Doer == nil || !isAdmin {
		repoIDs, err = repo_model.FindUserCodeAccessibleRepoIDs(ctx, ctx.Doer)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
	}

	var (
		total         int
		searchResults []*code_indexer.Result
		repoMaps      map[int64]*repo_model.Repository
	)

	if (len(repoIDs) > 0) || isAdmin {
		total, searchResults, _, err = code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
			RepoIDs:        repoIDs,
			Keyword:        keyword,
			IsKeywordFuzzy: isFuzzy,
			IsHtmlSafe:     false,
			Paginator: &db.ListOptions{
				Page:     page,
				PageSize: setting.API.DefaultPagingNum,
			},
		})
		if err != nil {
			if code_indexer.IsAvailable(ctx) {
				ctx.JSON(http.StatusInternalServerError, api.SearchError{
					OK:    false,
					Error: err.Error(),
				})
				return
			}
		}

		loadRepoIDs := make([]int64, 0, len(searchResults))
		for _, result := range searchResults {
			var find bool
			for _, id := range loadRepoIDs {
				if id == result.RepoID {
					find = true
					break
				}
			}
			if !find {
				loadRepoIDs = append(loadRepoIDs, result.RepoID)
			}
		}

		repoMaps, err = repo_model.GetRepositoriesMapByIDs(ctx, loadRepoIDs)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		if len(loadRepoIDs) != len(repoMaps) {
			// Remove deleted repos from search results
			cleanedSearchResults := make([]*code_indexer.Result, 0, len(repoMaps))
			for _, sr := range searchResults {
				if _, found := repoMaps[sr.RepoID]; found {
					cleanedSearchResults = append(cleanedSearchResults, sr)
				}
			}

			searchResults = cleanedSearchResults
		}
	}

	ctx.JSON(http.StatusOK, convert.ToExploreCodeSearchResults(total, searchResults, repoMaps))
}
