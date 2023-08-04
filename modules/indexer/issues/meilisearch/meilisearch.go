// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"strconv"
	"strings"

	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_meilisearch "code.gitea.io/gitea/modules/indexer/internal/meilisearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"github.com/meilisearch/meilisearch-go"
)

const (
	issueIndexerLatestVersion = 2

	// TODO: make this configurable if necessary
	maxTotalHits = 10000
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_meilisearch.Indexer
	indexer_internal.Indexer // do not composite inner_meilisearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new meilisearch indexer
func NewIndexer(url, apiKey, indexerName string) *Indexer {
	settings := &meilisearch.Settings{
		// The default ranking rules of meilisearch are: ["words", "typo", "proximity", "attribute", "sort", "exactness"]
		// So even if we specify the sort order, it could not be respected because the priority of "sort" is so low.
		// So we need to specify the ranking rules to make sure the sort order is respected.
		// See https://www.meilisearch.com/docs/learn/core_concepts/relevancy
		RankingRules: []string{"sort", // make sure "sort" has the highest priority
			"words", "typo", "proximity", "attribute", "exactness"},

		SearchableAttributes: []string{
			"title",
			"content",
			"comments",
		},
		DisplayedAttributes: []string{
			"id",
		},
		FilterableAttributes: []string{
			"repo_id",
			"is_public",
			"is_pull",
			"is_closed",
			"label_ids",
			"no_label",
			"milestone_id",
			"project_id",
			"project_board_id",
			"poster_id",
			"assignee_id",
			"mention_ids",
			"reviewed_ids",
			"review_requested_ids",
			"subscriber_ids",
			"updated_unix",
		},
		SortableAttributes: []string{
			"updated_unix",
			"created_unix",
			"deadline_unix",
			"comment_count",
			"id",
		},
		Pagination: &meilisearch.Pagination{
			MaxTotalHits: maxTotalHits,
		},
	}

	inner := inner_meilisearch.NewIndexer(url, apiKey, indexerName, issueIndexerLatestVersion, settings)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

// Index will save the index data
func (b *Indexer) Index(_ context.Context, issues ...*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	}
	for _, issue := range issues {
		_, err := b.inner.Client.Index(b.inner.VersionedIndexName()).AddDocuments(issue)
		if err != nil {
			return err
		}
	}
	// TODO: bulk send index data
	return nil
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(_ context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		_, err := b.inner.Client.Index(b.inner.VersionedIndexName()).DeleteDocument(strconv.FormatInt(id, 10))
		if err != nil {
			return err
		}
	}
	// TODO: bulk send deletes
	return nil
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	query := inner_meilisearch.FilterAnd{}

	if len(options.RepoIDs) > 0 {
		q := &inner_meilisearch.FilterOr{}
		q.Or(inner_meilisearch.NewFilterIn("repo_id", options.RepoIDs...))
		if options.AllPublic {
			q.Or(inner_meilisearch.NewFilterEq("is_public", true))
		}
		query.And(q)
	}

	if !options.IsPull.IsNone() {
		query.And(inner_meilisearch.NewFilterEq("is_pull", options.IsPull.IsTrue()))
	}
	if !options.IsClosed.IsNone() {
		query.And(inner_meilisearch.NewFilterEq("is_closed", options.IsClosed.IsTrue()))
	}

	if options.NoLabelOnly {
		query.And(inner_meilisearch.NewFilterEq("no_label", true))
	} else {
		if len(options.IncludedLabelIDs) > 0 {
			q := &inner_meilisearch.FilterAnd{}
			for _, labelID := range options.IncludedLabelIDs {
				q.And(inner_meilisearch.NewFilterEq("label_ids", labelID))
			}
			query.And(q)
		} else if len(options.IncludedAnyLabelIDs) > 0 {
			query.And(inner_meilisearch.NewFilterIn("label_ids", options.IncludedAnyLabelIDs...))
		}
		if len(options.ExcludedLabelIDs) > 0 {
			q := &inner_meilisearch.FilterAnd{}
			for _, labelID := range options.ExcludedLabelIDs {
				q.And(inner_meilisearch.NewFilterNot(inner_meilisearch.NewFilterEq("label_ids", labelID)))
			}
			query.And(q)
		}
	}

	if len(options.MilestoneIDs) > 0 {
		query.And(inner_meilisearch.NewFilterIn("milestone_id", options.MilestoneIDs...))
	}

	if options.ProjectID != nil {
		query.And(inner_meilisearch.NewFilterEq("project_id", *options.ProjectID))
	}
	if options.ProjectBoardID != nil {
		query.And(inner_meilisearch.NewFilterEq("project_board_id", *options.ProjectBoardID))
	}

	if options.PosterID != nil {
		query.And(inner_meilisearch.NewFilterEq("poster_id", *options.PosterID))
	}

	if options.AssigneeID != nil {
		query.And(inner_meilisearch.NewFilterEq("assignee_id", *options.AssigneeID))
	}

	if options.MentionID != nil {
		query.And(inner_meilisearch.NewFilterEq("mention_ids", *options.MentionID))
	}

	if options.ReviewedID != nil {
		query.And(inner_meilisearch.NewFilterEq("reviewed_ids", *options.ReviewedID))
	}
	if options.ReviewRequestedID != nil {
		query.And(inner_meilisearch.NewFilterEq("review_requested_ids", *options.ReviewRequestedID))
	}

	if options.SubscriberID != nil {
		query.And(inner_meilisearch.NewFilterEq("subscriber_ids", *options.SubscriberID))
	}

	if options.UpdatedAfterUnix != nil {
		query.And(inner_meilisearch.NewFilterGte("updated_unix", *options.UpdatedAfterUnix))
	}
	if options.UpdatedBeforeUnix != nil {
		query.And(inner_meilisearch.NewFilterLte("updated_unix", *options.UpdatedBeforeUnix))
	}

	if options.SortBy == "" {
		options.SortBy = internal.SortByCreatedAsc
	}
	sortBy := []string{
		parseSortBy(options.SortBy),
		"id:desc",
	}

	skip, limit := indexer_internal.ParsePaginator(options.Paginator, maxTotalHits)

	searchRes, err := b.inner.Client.Index(b.inner.VersionedIndexName()).Search(options.Keyword, &meilisearch.SearchRequest{
		Filter: query.Statement(),
		Limit:  int64(limit),
		Offset: int64(skip),
		Sort:   sortBy,
	})
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, len(searchRes.Hits))
	for _, hit := range searchRes.Hits {
		hits = append(hits, internal.Match{
			ID: int64(hit.(map[string]any)["id"].(float64)),
		})
	}

	return &internal.SearchResult{
		Total: searchRes.EstimatedTotalHits,
		Hits:  hits,
	}, nil
}

func parseSortBy(sortBy internal.SortBy) string {
	field := strings.TrimPrefix(string(sortBy), "-")
	if strings.HasPrefix(string(sortBy), "-") {
		return field + ":desc"
	}
	return field + ":asc"
}
