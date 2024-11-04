// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"

	"github.com/olivere/elastic/v7"
)

const (
	conversationIndexerLatestVersion = 1
	// multi-match-types, currently only 2 types are used
	// Reference: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-multi-match-query.html#multi-match-types
	esMultiMatchTypeBestFields   = "best_fields"
	esMultiMatchTypePhrasePrefix = "phrase_prefix"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_elasticsearch.Indexer
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	inner := inner_elasticsearch.NewIndexer(url, indexerName, conversationIndexerLatestVersion, defaultMapping)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

const (
	defaultMapping = `
{
	"mappings": {
		"properties": {
			"id": { "type": "integer", "index": true },
			"repo_id": { "type": "integer", "index": true },
			"is_public": { "type": "boolean", "index": true },

			"title": {  "type": "text", "index": true },
			"content": { "type": "text", "index": true },
			"comments": { "type" : "text", "index": true },

			"is_pull": { "type": "boolean", "index": true },
			"is_closed": { "type": "boolean", "index": true },
			"label_ids": { "type": "integer", "index": true },
			"no_label": { "type": "boolean", "index": true },
			"milestone_id": { "type": "integer", "index": true },
			"project_id": { "type": "integer", "index": true },
			"project_board_id": { "type": "integer", "index": true },
			"poster_id": { "type": "integer", "index": true },
			"assignee_id": { "type": "integer", "index": true },
			"mention_ids": { "type": "integer", "index": true },
			"reviewed_ids": { "type": "integer", "index": true },
			"review_requested_ids": { "type": "integer", "index": true },
			"subscriber_ids": { "type": "integer", "index": true },
			"updated_unix": { "type": "integer", "index": true },

			"created_unix": { "type": "integer", "index": true },
			"deadline_unix": { "type": "integer", "index": true },
			"comment_count": { "type": "integer", "index": true }
		}
	}
}
`
)

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, conversations ...*internal.IndexerData) error {
	if len(conversations) == 0 {
		return nil
	} else if len(conversations) == 1 {
		conversation := conversations[0]
		_, err := b.inner.Client.Index().
			Index(b.inner.VersionedIndexName()).
			Id(fmt.Sprintf("%d", conversation.ID)).
			BodyJson(conversation).
			Do(ctx)
		return err
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, conversation := range conversations {
		reqs = append(reqs,
			elastic.NewBulkIndexRequest().
				Index(b.inner.VersionedIndexName()).
				Id(fmt.Sprintf("%d", conversation.ID)).
				Doc(conversation),
		)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.VersionedIndexName()).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return err
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	} else if len(ids) == 1 {
		_, err := b.inner.Client.Delete().
			Index(b.inner.VersionedIndexName()).
			Id(fmt.Sprintf("%d", ids[0])).
			Do(ctx)
		return err
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, id := range ids {
		reqs = append(reqs,
			elastic.NewBulkDeleteRequest().
				Index(b.inner.VersionedIndexName()).
				Id(fmt.Sprintf("%d", id)),
		)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.VersionedIndexName()).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return err
}

// Search searches for conversations by given conditions.
// Returns the matching conversation IDs
func (b *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	query := elastic.NewBoolQuery()

	if options.Keyword != "" {
		searchType := esMultiMatchTypePhrasePrefix
		if options.IsFuzzyKeyword {
			searchType = esMultiMatchTypeBestFields
		}

		query.Must(elastic.NewMultiMatchQuery(options.Keyword, "title", "content", "comments").Type(searchType))
	}

	if len(options.RepoIDs) > 0 {
		q := elastic.NewBoolQuery()
		q.Should(elastic.NewTermsQuery("repo_id", toAnySlice(options.RepoIDs)...))
		if options.AllPublic {
			q.Should(elastic.NewTermQuery("is_public", true))
		}
		query.Must(q)
	}

	if options.UpdatedAfterUnix.Has() || options.UpdatedBeforeUnix.Has() {
		q := elastic.NewRangeQuery("updated_unix")
		if options.UpdatedAfterUnix.Has() {
			q.Gte(options.UpdatedAfterUnix.Value())
		}
		if options.UpdatedBeforeUnix.Has() {
			q.Lte(options.UpdatedBeforeUnix.Value())
		}
		query.Must(q)
	}

	if options.SortBy == "" {
		options.SortBy = internal.SortByCreatedAsc
	}
	sortBy := []elastic.Sorter{
		parseSortBy(options.SortBy),
		elastic.NewFieldSort("id").Desc(),
	}

	// See https://stackoverflow.com/questions/35206409/elasticsearch-2-1-result-window-is-too-large-index-max-result-window/35221900
	// TODO: make it configurable since it's configurable in elasticsearch
	const maxPageSize = 10000

	skip, limit := indexer_internal.ParsePaginator(options.Paginator, maxPageSize)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		SortBy(sortBy...).
		From(skip).Size(limit).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		id, _ := strconv.ParseInt(hit.Id, 10, 64)
		hits = append(hits, internal.Match{
			ID: id,
		})
	}

	return &internal.SearchResult{
		Total: searchResult.TotalHits(),
		Hits:  hits,
	}, nil
}

func toAnySlice[T any](s []T) []any {
	ret := make([]any, 0, len(s))
	for _, item := range s {
		ret = append(ret, item)
	}
	return ret
}

func parseSortBy(sortBy internal.SortBy) elastic.Sorter {
	field := strings.TrimPrefix(string(sortBy), "-")
	ret := elastic.NewFieldSort(field)
	if strings.HasPrefix(string(sortBy), "-") {
		ret.Desc()
	}
	return ret
}
