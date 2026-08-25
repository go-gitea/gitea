// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/container"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// GlobalSearch search codes in all accessible repositories with the given keyword.
func GlobalSearch(ctx *context.APIContext) {
	// swagger:operation GET /search/code search GlobalSearch
	// ---
	// summary: Search for code
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only in the repositories with the given full names
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: mode
	//   in: query
	//   description: search mode
	//   type: string
	//   enum: [exact, words, fuzzy, regexp]
	// - name: language
	//   in: query
	//   description: filter by programming language
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CodeSearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !setting.Indexer.RepoIndexerEnabled {
		ctx.APIError(http.StatusBadRequest, "Repository indexing is disabled")
		return
	}

	q := ctx.FormTrim("q")
	if q == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "Query cannot be empty")
		return
	}

	searchUser := ctx.Doer
	if ctx.PublicOnly {
		searchUser = nil
	}
	isAdmin := searchUser != nil && searchUser.IsAdmin

	// guest user or non-admin user
	var accessibleRepoIDs []int64
	var err error
	if !isAdmin {
		accessibleRepoIDs, err = repo_model.FindUserCodeAccessibleRepoIDs(ctx, searchUser)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	repoNames := ctx.FormStrings("repo")
	if len(repoNames) > 0 {
		searchRepoIDs, err := repo_model.GetRepositoriesIDsByFullNames(ctx, repoNames)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		if !isAdmin {
			accessibleSet := make(container.Set[int64], len(accessibleRepoIDs))
			for _, repoID := range accessibleRepoIDs {
				accessibleSet.Add(repoID)
			}
			filteredRepoIDs := make([]int64, 0, len(searchRepoIDs))
			for _, repoID := range searchRepoIDs {
				if accessibleSet.Contains(repoID) {
					filteredRepoIDs = append(filteredRepoIDs, repoID)
				}
			}
			searchRepoIDs = filteredRepoIDs
		}

		accessibleRepoIDs = searchRepoIDs
	}
	if !isAdmin && len(accessibleRepoIDs) == 0 {
		ctx.SetTotalCountHeader(0)
		ctx.JSON(http.StatusOK, structs.CodeSearchResults{Items: []structs.CodeSearchResult{}})
		return
	}

	searchMode := indexer.SearchModeType(ctx.FormString("mode"))
	listOpts := utils.GetListOptions(ctx)

	total, results, languages, err := code.PerformSearch(ctx, &code.SearchOptions{
		Keyword:     q,
		RepoIDs:     accessibleRepoIDs,
		Language:    ctx.FormString("language"),
		SearchMode:  searchMode,
		Paginator:   &listOpts,
		NoHighlight: true, // Default to no highlighting for performance, we don't need to highlight in the API search results
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(total)
	searchResults := structs.CodeSearchResults{
		TotalCount: total,
	}

	for _, lang := range languages {
		searchResults.Languages = append(searchResults.Languages, structs.CodeSearchResultLanguage{
			Language: lang.Language,
			Color:    lang.Color,
			Count:    lang.Count,
		})
	}

	repoIDs := make(container.Set[int64], len(results))
	for _, result := range results {
		repoIDs.Add(result.RepoID)
	}

	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs.Values())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	permissions := make(map[int64]access_model.Permission)
	for _, repo := range repos {
		permission, err := access_model.GetDoerRepoPermission(ctx, repo, searchUser)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		permissions[repo.ID] = permission
	}

	for _, result := range results {
		repo, ok := repos[result.RepoID]
		if !ok {
			log.Error("Repository with ID %d not found for search result: %v", result.RepoID, result)
			continue
		}

		apiURL := fmt.Sprintf("%s/contents/%s?ref=%s", repo.APIURL(), util.PathEscapeSegments(result.Filename), url.PathEscape(result.CommitID))
		htmlURL := fmt.Sprintf("%s/blob/%s/%s", repo.HTMLURL(), url.PathEscape(result.CommitID), util.PathEscapeSegments(result.Filename))
		ret := structs.CodeSearchResult{
			Name:       path.Base(result.Filename),
			Path:       result.Filename,
			Sha:        result.CommitID,
			URL:        apiURL,
			HTMLURL:    htmlURL,
			Language:   result.Language,
			Color:      result.Color,
			Repository: convert.ToRepo(ctx, repo, permissions[repo.ID]),
		}
		for _, line := range result.Lines {
			ret.Lines = append(ret.Lines, structs.CodeSearchResultLine{
				LineNumber: line.Num,
				RawContent: line.RawContent,
			})
		}
		searchResults.Items = append(searchResults.Items, ret)
	}

	ctx.JSON(200, searchResults)
}
