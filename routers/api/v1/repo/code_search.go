// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"path"
	"slices"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/gitgrep"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// SearchCode searches the code of a repository
func SearchCode(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/code/search repository repoSearchCode
	// ---
	// summary: Search the code of a repository
	// description: The files of the ref are searched with git grep, which returns at most 50 files.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keywords to search for
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch"
	//   type: string
	// - name: search_mode
	//   in: query
	//   description: how to match the keywords
	//   type: string
	//   enum: [exact, words, regexp]
	//   default: exact
	// - name: path
	//   in: query
	//   description: only search the files in this directory and its subdirectories
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	keyword := ctx.FormTrim("q")
	if keyword == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "q is required")
		return
	}

	searchMode := indexer.SearchModeType(util.IfZero(ctx.FormTrim("search_mode"), string(indexer.SearchModeExact)))
	if !slices.ContainsFunc(indexer.GitGrepSupportedSearchModes(), func(m indexer.SearchMode) bool { return m.ModeValue == searchMode }) {
		ctx.APIError(http.StatusUnprocessableEntity, "unsupported search_mode")
		return
	}

	refCommit := resolveRefCommit(ctx, ctx.FormTrim("ref"))
	if ctx.Written() {
		return
	}

	listOptions := utils.GetListOptions(ctx)
	results, total, incomplete, err := gitgrep.PerformSearch(ctx, ctx.Repo.GitRepo, &gitgrep.SearchOptions{
		RepoID:     ctx.Repo.Repository.ID,
		Ref:        git.RefNameFromCommit(refCommit.CommitID),
		Keyword:    keyword,
		SearchMode: searchMode,
		Path:       strings.Trim(path.Clean("/"+ctx.FormTrim("path")), "/"),
		Page:       listOptions.Page,
		PageSize:   listOptions.PageSize,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiRepo := convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission)
	items := make([]*api.CodeSearchResultItem, 0, len(results))
	for _, result := range results {
		items = append(items, convert.ToCodeSearchResultItem(ctx, ctx.Repo.Repository, apiRepo, result))
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &api.CodeSearchResults{
		TotalCount:        total,
		IncompleteResults: incomplete,
		Items:             items,
	})
}
