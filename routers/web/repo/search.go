// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

const tplSearch base.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	language := ctx.FormTrim("l")
	keyword := ctx.FormTrim("q")

	queryType := ctx.FormTrim("t")
	isMatch := queryType == "match"

	ctx.Data["Keyword"] = keyword
	ctx.Data["Language"] = language
	ctx.Data["queryType"] = queryType
	ctx.Data["PageIsViewCode"] = true

	if keyword == "" {
		ctx.HTML(http.StatusOK, tplSearch)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	total, searchResults, searchResultLanguages, err := code_indexer.PerformSearch(ctx, []int64{ctx.Repo.Repository.ID},
		language, keyword, page, setting.UI.RepoSearchPagingNum, isMatch)
	if err != nil {
		if code_indexer.IsAvailable() {
			ctx.ServerError("SearchResults", err)
			return
		}
		ctx.Data["CodeIndexerUnavailable"] = true
	} else {
		ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable()
	}

	ctx.Data["SourcePath"] = ctx.Repo.Repository.HTMLURL()
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "l", "Language")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSearch)
}

// SearchRepo repositories via options
func SearchRepo(ctx *context.Context) {
	opts := &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     ctx.FormInt("page"),
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		Actor:              ctx.Doer,
		Keyword:            ctx.FormTrim("q"),
		OwnerID:            ctx.FormInt64("uid"),
		PriorityOwnerID:    ctx.FormInt64("priority_owner_id"),
		TeamID:             ctx.FormInt64("team_id"),
		TopicOnly:          ctx.FormBool("topic"),
		Collaborate:        util.OptionalBoolNone,
		Private:            ctx.IsSigned && (ctx.FormString("private") == "" || ctx.FormBool("private")),
		Template:           util.OptionalBoolNone,
		StarredByID:        ctx.FormInt64("starredBy"),
		IncludeDescription: ctx.FormBool("includeDesc"),
	}

	if ctx.FormString("template") != "" {
		opts.Template = util.OptionalBoolOf(ctx.FormBool("template"))
	}

	if ctx.FormBool("exclusive") {
		opts.Collaborate = util.OptionalBoolFalse
	}

	mode := ctx.FormString("mode")
	switch mode {
	case "source":
		opts.Fork = util.OptionalBoolFalse
		opts.Mirror = util.OptionalBoolFalse
	case "fork":
		opts.Fork = util.OptionalBoolTrue
	case "mirror":
		opts.Mirror = util.OptionalBoolTrue
	case "collaborative":
		opts.Mirror = util.OptionalBoolFalse
		opts.Collaborate = util.OptionalBoolTrue
	case "":
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid search mode: \"%s\"", mode))
		return
	}

	if ctx.FormString("archived") != "" {
		opts.Archived = util.OptionalBoolOf(ctx.FormBool("archived"))
	}

	if ctx.FormString("is_private") != "" {
		opts.IsPrivate = util.OptionalBoolOf(ctx.FormBool("is_private"))
	}

	sortMode := ctx.FormString("sort")
	if len(sortMode) > 0 {
		sortOrder := ctx.FormString("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := context.SearchOrderByMap[sortOrder]; ok {
			if orderBy, ok := searchModeMap[sortMode]; ok {
				opts.OrderBy = orderBy
			} else {
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid sort mode: \"%s\"", sortMode))
				return
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid sort order: \"%s\"", sortOrder))
			return
		}
	}

	var err error
	repos, count, err := repo_model.SearchRepository(opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	ctx.SetTotalCountHeader(count)

	// To improve performance when only the count is requested
	if ctx.FormBool("count_only") {
		return
	}

	results := make([]*api.Repository, len(repos))
	for i, repo := range repos {
		results[i] = &api.Repository{
			ID:       repo.ID,
			FullName: repo.FullName(),
			Fork:     repo.IsFork,
			Private:  repo.IsPrivate,
			Template: repo.IsTemplate,
			Mirror:   repo.IsMirror,
			Stars:    repo.NumStars,
			HTMLURL:  repo.HTMLURL(),
			Internal: !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		}
	}

	ctx.JSON(http.StatusOK, api.SearchResults{
		OK:   true,
		Data: results,
	})
}
