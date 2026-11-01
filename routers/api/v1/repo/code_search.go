// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/codesearch"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

const codeSearchResultWindow = 1000

func codeSearchError(ctx *context.APIContext, err error) {
	if queryError, ok := errors.AsType[*indexer.SearchQueryError](err); ok {
		ctx.APIError(http.StatusUnprocessableEntity, queryError.Message)
	} else if errors.Is(err, codesearch.ErrIndexerUnavailable) {
		ctx.APIError(http.StatusServiceUnavailable, err.Error())
	} else {
		ctx.APIErrorInternal(err)
	}
}

func respondCodeSearch(ctx *context.APIContext, opts *codesearch.Options) {
	listOptions := utils.GetListOptions(ctx)
	if listOptions.Page > codeSearchResultWindow/listOptions.PageSize {
		codeSearchError(ctx, &indexer.SearchQueryError{Message: "page * limit must not exceed 1000"})
		return
	}
	opts.Paginator = &listOptions
	result, err := codesearch.Search(ctx, ctx.FormTrim("q"), opts)
	if err != nil {
		codeSearchError(ctx, err)
		return
	}
	if opts.RepoID != 0 {
		result.Repos = map[int64]*repo_model.Repository{opts.RepoID: ctx.Repo.Repository}
	}
	apiRepos := make(map[int64]*api.Repository, len(result.Repos))
	for _, repo := range result.Repos {
		permission := ctx.Repo.Permission
		if opts.RepoID == 0 {
			permission, err = access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
		}
		if permission.CanRead(unit.TypeCode) {
			apiRepos[repo.ID] = convert.ToRepo(ctx, repo, permission)
		}
	}
	items := make([]*api.CodeSearchResultItem, 0, len(result.Results))
	for _, item := range result.Results {
		if apiRepo := apiRepos[item.RepoID]; apiRepo != nil {
			items = append(items, convert.ToCodeSearchResultItem(ctx, result.Repos[item.RepoID], apiRepo, item))
		}
	}
	ctx.SetLinkHeader(min(result.Total, int64(codeSearchResultWindow/opts.Paginator.PageSize*opts.Paginator.PageSize)), opts.Paginator.PageSize)
	ctx.SetTotalCountHeader(result.Total)
	ctx.JSON(http.StatusOK, &api.CodeSearchResults{
		TotalCount:        result.Total,
		IncompleteResults: result.Incomplete,
		Items:             items,
	})
}

// SearchRepoCode searches the code of a repository
func SearchRepoCode(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/code/search repository repoSearchCode
	// ---
	// summary: Search the code of a repository
	// description: The files of the ref are searched with git grep, which returns at most 50 files. The result window is limited to 1000 files (page * limit).
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
	//   description: 'Terms must all occur in a file. Supports "exact phrases", /regexp/ and path:DIR. Language and repository qualifiers are not supported.'
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch"
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
	//   default:
	//     "$ref": "#/responses/error"

	opts := &codesearch.Options{Engine: codesearch.GitGrep, RepoID: ctx.Repo.Repository.ID, GitRepo: ctx.Repo.GitRepo}
	ref := ctx.FormTrim("ref")
	if !ctx.Repo.Repository.IsEmpty || ref != "" {
		refCommit := resolveRefCommit(ctx, ref)
		if ctx.Written() {
			return
		}
		opts.Ref = git.RefNameFromCommit(refCommit.CommitID)
	}
	respondCodeSearch(ctx, opts)
}

// SearchCode searches the code of all accessible repositories
func SearchCode(ctx *context.APIContext) {
	// swagger:operation GET /search/code repository codeSearch
	// ---
	// summary: Search the code of all repositories
	// description: Authentication is required. The default branches are searched with the code indexer. Regular expressions are not supported. The result window is limited to 1000 files (page * limit).
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: |
	//     Literal terms and "exact phrases" must all occur in a file, with optional qualifiers:
	//     "repo:OWNER/NAME", "user:NAME" and "org:NAME" to only search these repositories (each can be repeated),
	//     "path:DIR" to only search the files in a directory, "language:NAME" to only search the files of a language
	//   type: string
	//   required: true
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
	//   default:
	//     "$ref": "#/responses/error"

	respondCodeSearch(ctx, &codesearch.Options{Engine: codesearch.Indexer, Doer: ctx.Doer})
}
