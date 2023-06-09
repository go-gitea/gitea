// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"github.com/meilisearch/meilisearch-go"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	client      *meilisearch.Client
	indexerName string
	available   bool
	stopTimer   chan struct{}
	lock        sync.RWMutex
}

// Indexer creates a new meilisearch indexer
func NewMeilisearchIndexer(url, apiKey, indexerName string) (*Indexer, error) {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   url,
		APIKey: apiKey,
	})

	indexer := &Indexer{
		client:      client,
		indexerName: indexerName,
		available:   true,
		stopTimer:   make(chan struct{}),
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				indexer.checkAvailability()
			case <-indexer.stopTimer:
				ticker.Stop()
				return
			}
		}
	}()

	return indexer, nil
}

// Init will initialize the indexer
func (b *Indexer) Init() (bool, error) {
	_, err := b.client.GetIndex(b.indexerName)
	if err == nil {
		return true, nil
	}
	_, err = b.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        b.indexerName,
		PrimaryKey: "id",
	})
	if err != nil {
		return false, b.checkError(err)
	}

	_, err = b.client.Index(b.indexerName).UpdateFilterableAttributes(&[]string{"repo_id"})
	return false, b.checkError(err)
}

// Ping checks if meilisearch is available
func (b *Indexer) Ping() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.available
}

// Index will save the index data
func (b *Indexer) Index(issues []*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	}
	for _, issue := range issues {
		_, err := b.client.Index(b.indexerName).AddDocuments(issue)
		if err != nil {
			return b.checkError(err)
		}
	}
	// TODO: bulk send index data
	return nil
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		_, err := b.client.Index(b.indexerName).DeleteDocument(strconv.FormatInt(id, 10))
		if err != nil {
			return b.checkError(err)
		}
	}
	// TODO: bulk send deletes
	return nil
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*internal.SearchResult, error) {
	repoFilters := make([]string, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		repoFilters = append(repoFilters, "repo_id = "+strconv.FormatInt(repoID, 10))
	}
	filter := strings.Join(repoFilters, " OR ")
	searchRes, err := b.client.Index(b.indexerName).Search(keyword, &meilisearch.SearchRequest{
		Filter: filter,
		Limit:  int64(limit),
		Offset: int64(start),
	})
	if err != nil {
		return nil, b.checkError(err)
	}

	hits := make([]internal.Match, 0, len(searchRes.Hits))
	for _, hit := range searchRes.Hits {
		hits = append(hits, internal.Match{
			ID: int64(hit.(map[string]interface{})["id"].(float64)),
		})
	}
	return &internal.SearchResult{
		Total: searchRes.TotalHits,
		Hits:  hits,
	}, nil
}

// Close implements indexer
func (b *Indexer) Close() {
	select {
	case <-b.stopTimer:
	default:
		close(b.stopTimer)
	}
}

func (b *Indexer) checkError(err error) error {
	return err
}

func (b *Indexer) checkAvailability() {
	_, err := b.client.Health()
	if err != nil {
		b.setAvailability(false)
		return
	}
	b.setAvailability(true)
}

func (b *Indexer) setAvailability(available bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.available == available {
		return
	}

	b.available = available
}
