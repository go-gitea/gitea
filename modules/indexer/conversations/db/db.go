// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	conversation_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"

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

// Search searches for conversations
func (i *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	// FIXME: I tried to avoid importing models here, but it seems to be impossible.
	//        We can provide a function to register the search function, so models/conversations can register it.
	//        So models/conversations will import modules/indexer/conversations, it's OK because it's by design.
	//        But modules/indexer/conversations has already imported models/conversations to do UpdateRepoIndexer and UpdateConversationIndexer.
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
		subQuery := builder.Select("id").From("conversation").Where(repoCond)

		cond = builder.Or(
			builder.In("conversation.id", builder.Select("id").
				From("comment").
				Where(builder.And(
					builder.Eq{"type": conversation_model.CommentTypeComment},
					builder.In("conversation_id", subQuery),
					db.BuildCaseInsensitiveLike("content", options.Keyword),
				)),
			),
		)

		if options.IsKeywordNumeric() {
			cond = cond.Or(
				builder.Eq{"`index`": options.Keyword},
			)
		}
	}

	opt, err := ToDBOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// If pagesize == 0, return total count only. It's a special case for search count.
	if options.Paginator != nil && options.Paginator.PageSize == 0 {
		total, err := conversation_model.CountConversations(ctx, opt, cond)
		if err != nil {
			return nil, err
		}
		return &internal.SearchResult{
			Total: total,
		}, nil
	}

	ids, total, err := conversation_model.ConversationIDs(ctx, opt, cond)
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
