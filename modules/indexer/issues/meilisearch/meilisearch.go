// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_meilisearch "code.gitea.io/gitea/modules/indexer/internal/meilisearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"github.com/meilisearch/meilisearch-go"
)

const (
	issueIndexerLatestVersion = 2
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_meilisearch.Indexer
	indexer_internal.Indexer // do not composite inner_meilisearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new meilisearch indexer
func NewIndexer(url, apiKey, indexerName string) *Indexer {
	inner := inner_meilisearch.NewIndexer(url, apiKey, indexerName, issueIndexerLatestVersion)
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
	repoFilters := make([]string, 0, len(options.RepoIDs))
	for _, repoID := range options.RepoIDs {
		repoFilters = append(repoFilters, "repo_id = "+strconv.FormatInt(repoID, 10))
	}
	filter := strings.Join(repoFilters, " OR ")
	skip, limit := indexer_internal.ParsePaginator(options.Paginator)

	if !options.IsClosed.IsNone() {
		condition := fmt.Sprintf("is_closed = %t", options.IsClosed.IsTrue())
		if filter != "" {
			filter = "(" + filter + ") AND " + condition
		} else {
			filter = "state = " + condition
		}
	}

	// TODO: support more conditions

	searchRes, err := b.inner.Client.Index(b.inner.VersionedIndexName()).Search(options.Keyword, &meilisearch.SearchRequest{
		Filter: filter,
		Limit:  int64(limit),
		Offset: int64(skip),
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
		Total:     searchRes.TotalHits,
		Hits:      hits,
		Imprecise: true,
	}, nil
}
