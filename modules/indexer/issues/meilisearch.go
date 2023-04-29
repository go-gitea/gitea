// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

var _ Indexer = &MeilisearchIndexer{}

// MeilisearchIndexer implements Indexer interface
type MeilisearchIndexer struct {
	client               *meilisearch.Client
	indexerName          string
	available            bool
	availabilityCallback func(bool)
	stopTimer            chan struct{}
	lock                 sync.RWMutex
}

// MeilisearchIndexer creates a new meilisearch indexer
func NewMeilisearchIndexer(url, apiKey, indexerName string) (*MeilisearchIndexer, error) {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   url,
		APIKey: apiKey,
	})

	indexer := &MeilisearchIndexer{
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
func (b *MeilisearchIndexer) Init() (bool, error) {
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

// SetAvailabilityChangeCallback sets callback that will be triggered when availability changes
func (b *MeilisearchIndexer) SetAvailabilityChangeCallback(callback func(bool)) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.availabilityCallback = callback
}

// Ping checks if meilisearch is available
func (b *MeilisearchIndexer) Ping() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.available
}

// Index will save the index data
func (b *MeilisearchIndexer) Index(issues []*IndexerData) error {
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
func (b *MeilisearchIndexer) Delete(ids ...int64) error {
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
func (b *MeilisearchIndexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*SearchResult, error) {
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

	hits := make([]Match, 0, len(searchRes.Hits))
	for _, hit := range searchRes.Hits {
		hits = append(hits, Match{
			ID: int64(hit.(map[string]interface{})["id"].(float64)),
		})
	}
	return &SearchResult{
		Total: searchRes.TotalHits,
		Hits:  hits,
	}, nil
}

// Close implements indexer
func (b *MeilisearchIndexer) Close() {
	select {
	case <-b.stopTimer:
	default:
		close(b.stopTimer)
	}
}

func (b *MeilisearchIndexer) checkError(err error) error {
	return err
}

func (b *MeilisearchIndexer) checkAvailability() {
	_, err := b.client.Health()
	if err != nil {
		b.setAvailability(false)
		return
	}
	b.setAvailability(true)
}

func (b *MeilisearchIndexer) setAvailability(available bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.available == available {
		return
	}

	b.available = available
	if b.availabilityCallback != nil {
		// Call the callback from within the lock to ensure that the ordering remains correct
		b.availabilityCallback(b.available)
	}
}
