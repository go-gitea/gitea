// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/graceful"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"github.com/olivere/elastic/v7"
)

const (
	issueIndexerLatestVersion = 0
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_elasticsearch.Indexer
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	inner := inner_elasticsearch.NewIndexer(url, indexerName, issueIndexerLatestVersion, defaultMapping)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

const (
	defaultMapping = `{
		"mappings": {
			"properties": {
				"id": {
					"type": "integer",
					"index": true
				},
				"repo_id": {
					"type": "integer",
					"index": true
				},
				"title": {
					"type": "text",
					"index": true
				},
				"content": {
					"type": "text",
					"index": true
				},
				"comments": {
					"type" : "text",
					"index": true
				}
			}
		}
	}`
)

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, issues []*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]
		_, err := b.inner.Client.Index().
			Index(b.inner.VersionedIndexName()).
			Id(fmt.Sprintf("%d", issue.ID)).
			BodyJson(map[string]any{
				"id":       issue.ID,
				"repo_id":  issue.RepoID,
				"title":    issue.Title,
				"content":  issue.Content,
				"comments": issue.Comments,
			}).
			Do(ctx)
		return err
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, issue := range issues {
		reqs = append(reqs,
			elastic.NewBulkIndexRequest().
				Index(b.inner.VersionedIndexName()).
				Id(fmt.Sprintf("%d", issue.ID)).
				Doc(map[string]any{
					"id":       issue.ID,
					"repo_id":  issue.RepoID,
					"title":    issue.Title,
					"content":  issue.Content,
					"comments": issue.Comments,
				}),
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

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*internal.SearchResult, error) {
	kwQuery := elastic.NewMultiMatchQuery(keyword, "title", "content", "comments")
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(repoIDs) > 0 {
		repoStrs := make([]any, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Sort("_score", false).
		From(start).Size(limit).
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
