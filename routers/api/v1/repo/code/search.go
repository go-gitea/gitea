// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// GlobalSearch search codes in all accessible repositories with the given keyword.
func GlobalSearch(ctx *context.APIContext) {
	// swagger:operation GET /search/code search GlobalSearch
	// ---
	// summary: Search for repositories
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: repo
	//   in: query
	//   description: multiple repository names to search in
	//   type: string
	//   collectionFormat: multi
	// - name: mode
	//   in: query
	//   description: include search of keyword within repository description
	//   type: string
	//   enum: [exact, words, fuzzy, regexp]
	// - name: language
	//   in: query
	//   description: filter by programming language
	//   type: integer
	//   format: int64
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

	var (
		accessibleRepoIDs []int64
		err               error
		isAdmin           bool
	)
	if ctx.Doer != nil {
		isAdmin = ctx.Doer.IsAdmin
	}

	// guest user or non-admin user
	if ctx.Doer == nil || !isAdmin {
		accessibleRepoIDs, err = repo_model.FindUserCodeAccessibleRepoIDs(ctx, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	repoNames := ctx.FormStrings("repo")
	searchRepoIDs := make([]int64, 0, len(repoNames))
	if len(repoNames) > 0 {
		var err error
		searchRepoIDs, err = repo_model.GetRepositoriesIDsByFullNames(ctx, repoNames)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	if len(searchRepoIDs) > 0 {
		for i := 0; i < len(searchRepoIDs); i++ {
			if !slices.Contains(accessibleRepoIDs, searchRepoIDs[i]) {
				searchRepoIDs = append(searchRepoIDs[:i], searchRepoIDs[i+1:]...)
				i--
			}
		}
	}
	if len(searchRepoIDs) > 0 {
		accessibleRepoIDs = searchRepoIDs
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

	ctx.SetTotalCountHeader(int64(total))
	searchResults := structs.CodeSearchResults{
		TotalCount: int64(total),
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
		permission, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
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
