// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/gitgrep"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"

	"github.com/go-enry/go-enry/v2"
	"xorm.io/builder"
)

// codeSearchQualifiers are all qualifiers of code search queries, an endpoint rejects the ones it doesn't support
var codeSearchQualifiers = []string{"repo", "user", "org", "path", "language"}

// parseCodeSearchQuery parses "q", allowing each of the single qualifiers at most once
func parseCodeSearchQuery(ctx *context.APIContext, supported, single []string) *indexer.SearchQuery {
	query := indexer.ParseSearchQuery(ctx.FormTrim("q"), codeSearchQualifiers...)
	if query.Keyword == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "q must contain keywords to search for")
		return nil
	}
	for _, qualifier := range query.Qualifiers {
		if qualifier.Exclude || !slices.Contains(supported, qualifier.Name) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("qualifier %q is not supported here", util.Iif(qualifier.Exclude, "-", "")+qualifier.Name+":"))
			return nil
		}
	}
	for _, name := range single {
		if len(query.Values(name)) > 1 {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("qualifier %q can only be used once", name+":"))
			return nil
		}
	}
	return query
}

func parseCodeSearchMode(ctx *context.APIContext, supported []indexer.SearchMode) (indexer.SearchModeType, bool) {
	searchMode := indexer.SearchModeType(util.IfZero(ctx.FormTrim("search_mode"), string(indexer.SearchModeExact)))
	if !slices.ContainsFunc(supported, func(m indexer.SearchMode) bool { return m.ModeValue == searchMode }) {
		ctx.APIError(http.StatusUnprocessableEntity, "unsupported search_mode")
		return "", false
	}
	return searchMode, true
}

func firstQualifierPath(query *indexer.SearchQuery) string {
	paths := query.Values("path")
	if len(paths) == 0 {
		return ""
	}
	return strings.Trim(path.Clean("/"+paths[0]), "/")
}

// SearchRepoCode searches the code of a repository
func SearchRepoCode(ctx *context.APIContext) {
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
	//   description: keywords to search for, with an optional "path:DIR" qualifier to only search the files in a directory
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

	query := parseCodeSearchQuery(ctx, []string{"path"}, []string{"path"})
	if query == nil {
		return
	}
	searchMode, ok := parseCodeSearchMode(ctx, indexer.GitGrepSupportedSearchModes())
	if !ok {
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
		Keyword:    query.Keyword,
		SearchMode: searchMode,
		Path:       firstQualifierPath(query),
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

// SearchCode searches the code of all accessible repositories
func SearchCode(ctx *context.APIContext) {
	// swagger:operation GET /search/code repository codeSearch
	// ---
	// summary: Search the code of all repositories
	// description: The default branches are searched with the code indexer.
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: |
	//     keywords to search for, with optional qualifiers:
	//     "repo:OWNER/NAME", "user:NAME" and "org:NAME" to only search these repositories (each can be repeated),
	//     "path:DIR" to only search the files in a directory, "language:NAME" to only search the files of a language
	//   type: string
	//   required: true
	// - name: search_mode
	//   in: query
	//   description: how to match the keywords
	//   type: string
	//   enum: [exact, words]
	//   default: exact
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
	//   "503":
	//     "$ref": "#/responses/error"

	query := parseCodeSearchQuery(ctx, codeSearchQualifiers, []string{"path", "language"})
	if query == nil {
		return
	}
	if !code_indexer.IsAvailable(ctx) {
		ctx.APIError(http.StatusServiceUnavailable, "code indexer is unavailable")
		return
	}
	searchMode, ok := parseCodeSearchMode(ctx, code_indexer.SupportedSearchModes())
	if !ok {
		return
	}

	repoIDs, allRepos := codeSearchRepoIDs(ctx, query)
	if ctx.Written() {
		return
	}
	listOptions := utils.GetListOptions(ctx)
	if !allRepos && len(repoIDs) == 0 {
		ctx.SetTotalCountHeader(0)
		ctx.JSON(http.StatusOK, &api.CodeSearchResults{Items: []*api.CodeSearchResultItem{}})
		return
	}

	language := ""
	if languages := query.Values("language"); len(languages) > 0 {
		language, _ = enry.GetLanguageByAlias(languages[0]) // the index stores the canonical name, e.g. "Go" for "go" or "golang"
		language = util.IfZero(language, languages[0])
	}
	total, results, _, err := code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
		RepoIDs:    repoIDs,
		Keyword:    query.Keyword,
		Language:   language,
		Path:       firstQualifierPath(query),
		SearchMode: searchMode,
		Paginator:  &listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	resultRepoIDs := make(container.Set[int64])
	for _, result := range results {
		resultRepoIDs.Add(result.RepoID)
	}
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, resultRepoIDs.Values())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiRepos := make(map[int64]*api.Repository, len(repos))
	for _, repo := range repos {
		permission, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		apiRepos[repo.ID] = convert.ToRepo(ctx, repo, permission)
	}

	items := make([]*api.CodeSearchResultItem, 0, len(results))
	for _, result := range results {
		if repo, ok := repos[result.RepoID]; ok { // the index may still contain deleted repositories
			items = append(items, convert.ToCodeSearchResultItem(ctx, repo, apiRepos[repo.ID], result))
		}
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &api.CodeSearchResults{
		TotalCount: total,
		Items:      items,
	})
}

// codeSearchRepoIDs returns the IDs of the repositories to search, or allRepos for a site admin searching everything
func codeSearchRepoIDs(ctx *context.APIContext, query *indexer.SearchQuery) (repoIDs []int64, allRepos bool) {
	fullNames, ownerNames := query.Values("repo"), append(query.Values("user"), query.Values("org")...)
	for _, fullName := range fullNames {
		if ownerName, repoName, ok := strings.Cut(fullName, "/"); !ok || ownerName == "" || repoName == "" {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf(`qualifier "repo:" must be OWNER/NAME, got %q`, fullName))
			return nil, false
		}
	}

	isSiteAdmin := ctx.Doer != nil && ctx.Doer.IsAdmin && !ctx.PublicOnly
	if isSiteAdmin && len(fullNames) == 0 && len(ownerNames) == 0 {
		return nil, true
	}

	cond := builder.NewCond()
	if !isSiteAdmin {
		cond = cond.And(repo_model.AccessibleRepositoryCondition(ctx.Doer, unit.TypeCode))
	}
	if ctx.PublicOnly {
		cond = cond.And(repo_model.PublicRepoUnderPublicOwnerCond())
	}
	if len(fullNames) > 0 {
		cond = cond.And(repo_model.FullNamesCond(fullNames))
	}
	if len(ownerNames) > 0 {
		cond = cond.And(repo_model.OwnerNamesCond(ownerNames))
	}
	repoIDs, err := repo_model.SearchRepositoryIDsByCondition(ctx, cond)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	}
	return repoIDs, false
}
