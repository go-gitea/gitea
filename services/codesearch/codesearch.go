// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codesearch

import (
	"context"
	"errors"
	"slices"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/gitgrep"
)

var ErrIndexerUnavailable = errors.New("code indexer is unavailable")

type Engine int

const (
	Indexer Engine = iota
	GitGrep
)

type Options struct {
	Engine    Engine
	Doer      *user_model.User
	RepoID    int64
	OwnerName string
	GitRepo   *git.Repository
	Ref       git.RefName
	Paginator *db.ListOptions
}

type Result struct {
	Query      *indexer.SearchQuery
	Total      int64
	Results    []*code_indexer.Result
	Languages  []*code_indexer.SearchResultLanguages
	Repos      map[int64]*repo_model.Repository
	Incomplete bool
}

func Search(ctx context.Context, keyword string, opts *Options) (*Result, error) {
	query, err := indexer.ParseSearchQuery(keyword)
	if err != nil {
		return nil, err
	}
	switch {
	case opts.Engine == Indexer && len(query.Literals()) < len(query.Terms):
		return nil, &indexer.SearchQueryError{Message: "Regular expressions are not supported by the code indexer"}
	case opts.Engine == GitGrep && query.Language != "":
		return nil, &indexer.SearchQueryError{Message: "Language qualifiers are not supported by git grep"}
	case len(query.Repos)+len(query.Owners) > 0 && (opts.RepoID != 0 || opts.OwnerName != ""):
		return nil, &indexer.SearchQueryError{Message: "Repository and owner qualifiers are only supported in global code search"}
	}
	result := &Result{Query: query}
	if opts.Engine == GitGrep {
		result.Results, result.Total, result.Incomplete, err = gitgrep.PerformSearch(ctx, opts.GitRepo, &gitgrep.SearchOptions{
			RepoID: opts.RepoID, Ref: opts.Ref, Terms: query.Terms, Path: query.Path, Page: opts.Paginator.Page, PageSize: opts.Paginator.PageSize,
		})
		return result, err
	}
	if !code_indexer.IsAvailable(ctx) {
		return nil, ErrIndexerUnavailable
	}
	repoIDs := []int64{opts.RepoID}
	if opts.RepoID == 0 {
		owners := query.Owners
		if opts.OwnerName != "" {
			owners = []string{opts.OwnerName}
		}
		repoIDs, err = repo_model.FindCodeSearchRepoIDs(ctx, opts.Doer, query.Repos, owners)
		if err != nil || repoIDs != nil && len(repoIDs) == 0 {
			return result, err
		}
	}
	result.Total, result.Results, result.Languages, err = code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
		RepoIDs: repoIDs, Terms: query.Literals(), Language: query.Language, Path: query.Path, Paginator: opts.Paginator,
	})
	if err != nil || opts.RepoID != 0 {
		return result, err
	}
	result.Repos, err = repo_model.GetRepositoriesMapByIDs(ctx, container.FilterSlice(result.Results, func(item *code_indexer.Result) (int64, bool) {
		return item.RepoID, true
	}))
	result.Results = slices.DeleteFunc(result.Results, func(item *code_indexer.Result) bool {
		return result.Repos[item.RepoID] == nil // The index can still contain deleted repositories.
	})
	return result, err
}
