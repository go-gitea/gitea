// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"

	"github.com/olivere/elastic/v7"
)

var _ Indexer = &ElasticsearchIndexer{}

// ElasticsearchIndexer implements Indexer interface
type ElasticsearchIndexer struct {
	inner            *inner_elasticsearch.Indexer
	internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewElasticsearchIndexer creates a new elasticsearch indexer
func NewElasticsearchIndexer(url, indexerName string) *ElasticsearchIndexer {
	in := inner_elasticsearch.NewIndexer(url, indexerName, issueIndexerLatestVersion, defaultMapping)
	indexer := &ElasticsearchIndexer{
		inner:   in,
		Indexer: in,
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
func (b *ElasticsearchIndexer) Index(issues []*IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]
		_, err := b.inner.Client.Index().
			Index(b.inner.IndexName()).
			Id(fmt.Sprintf("%d", issue.ID)).
			BodyJson(map[string]interface{}{
				"id":       issue.ID,
				"repo_id":  issue.RepoID,
				"title":    issue.Title,
				"content":  issue.Content,
				"comments": issue.Comments,
			}).
			Do(graceful.GetManager().HammerContext())
		return b.inner.CheckError(err)
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, issue := range issues {
		reqs = append(reqs,
			elastic.NewBulkIndexRequest().
				Index(b.inner.IndexName()).
				Id(fmt.Sprintf("%d", issue.ID)).
				Doc(map[string]interface{}{
					"id":       issue.ID,
					"repo_id":  issue.RepoID,
					"title":    issue.Title,
					"content":  issue.Content,
					"comments": issue.Comments,
				}),
		)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.IndexName()).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return b.inner.CheckError(err)
}

// Delete deletes indexes by ids
func (b *ElasticsearchIndexer) Delete(ids ...int64) error {
	if len(ids) == 0 {
		return nil
	} else if len(ids) == 1 {
		_, err := b.inner.Client.Delete().
			Index(b.inner.IndexName()).
			Id(fmt.Sprintf("%d", ids[0])).
			Do(graceful.GetManager().HammerContext())
		return b.inner.CheckError(err)
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, id := range ids {
		reqs = append(reqs,
			elastic.NewBulkDeleteRequest().
				Index(b.inner.IndexName()).
				Id(fmt.Sprintf("%d", id)),
		)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.IndexName()).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return b.inner.CheckError(err)
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *ElasticsearchIndexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	kwQuery := elastic.NewMultiMatchQuery(keyword, "title", "content", "comments")
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(repoIDs) > 0 {
		repoStrs := make([]interface{}, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.IndexName()).
		Query(query).
		Sort("_score", false).
		From(start).Size(limit).
		Do(ctx)
	if err != nil {
		return nil, b.inner.CheckError(err)
	}

	hits := make([]Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		id, _ := strconv.ParseInt(hit.Id, 10, 64)
		hits = append(hits, Match{
			ID: id,
		})
	}

	return &SearchResult{
		Total: searchResult.TotalHits(),
		Hits:  hits,
	}, nil
}
