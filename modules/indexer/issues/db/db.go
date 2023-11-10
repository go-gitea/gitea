// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"xorm.io/builder"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface to use database's like search
type Indexer struct {
	indexer_internal.Indexer
}

func NewIndexer() *Indexer {
	return &Indexer{
		Indexer: &inner_db.Indexer{},
	}
}

// Index dummy function
func (i *Indexer) Index(_ context.Context, _ ...*internal.IndexerData) error {
	return nil
}

// Delete dummy function
func (i *Indexer) Delete(_ context.Context, _ ...int64) error {
	return nil
}

// Search searches for issues
func (i *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	// FIXME: I tried to avoid importing models here, but it seems to be impossible.
	//        We can provide a function to register the search function, so models/issues can register it.
	//        So models/issues will import modules/indexer/issues, it's OK because it's by design.
	//        But modules/indexer/issues has already imported models/issues to do UpdateRepoIndexer and UpdateIssueIndexer.
	//        And to avoid circular import, we have to move the functions to another package.
	//        I believe it should be services/indexer, sounds great!
	//        But the two functions are used in modules/notification/indexer, that means we will import services/indexer in modules/notification/indexer.
	//        So that's the root problem:
	//        The notification is defined in modules, but it's using lots of things should be in services.

	cond := builder.NewCond()

	if options.Keyword != "" {
		repoCond := builder.In("repo_id", options.RepoIDs)
		if len(options.RepoIDs) == 1 {
			repoCond = builder.Eq{"repo_id": options.RepoIDs[0]}
		}
		subQuery := builder.Select("id").From("issue").Where(repoCond)

		cond = builder.Or(
			db.BuildCaseInsensitiveLike("issue.name", options.Keyword),
			db.BuildCaseInsensitiveLike("issue.content", options.Keyword),
			builder.In("issue.id", builder.Select("issue_id").
				From("comment").
				Where(builder.And(
					builder.Eq{"type": issue_model.CommentTypeComment},
					builder.In("issue_id", subQuery),
					db.BuildCaseInsensitiveLike("content", options.Keyword),
				)),
			),
		)
	}

	opt, err := ToDBOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	ids, total, err := issue_model.IssueIDs(ctx, opt, cond)
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, len(ids))
	for _, id := range ids {
		hits = append(hits, internal.Match{
			ID: id,
		})
	}
	return &internal.SearchResult{
		Total: total,
		Hits:  hits,
	}, nil
}
