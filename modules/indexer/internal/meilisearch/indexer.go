// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

// Indexer represents a basic meilisearch indexer implementation
type Indexer struct {
	Client *meilisearch.Client

	url, apiKey string
	indexerName string
}

func NewIndexer(url, apiKey, indexerName string) *Indexer {
	return &Indexer{
		url:         url,
		apiKey:      apiKey,
		indexerName: indexerName,
	}
}

// Init initializes the indexer
func (i *Indexer) Init(_ context.Context) (bool, error) {
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}

	if i.Client != nil {
		return false, fmt.Errorf("indexer is already initialized")
	}

	i.Client = meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   i.url,
		APIKey: i.apiKey,
	})

	_, err := i.Client.GetIndex(i.indexerName)
	if err == nil {
		return true, nil
	}
	_, err = i.Client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        i.indexerName,
		PrimaryKey: "id",
	})
	if err != nil {
		return false, err
	}

	// TODO support version ?

	_, err = i.Client.Index(i.indexerName).UpdateFilterableAttributes(&[]string{"repo_id"})
	return false, err
}

// Ping checks if the indexer is available
func (i *Indexer) Ping(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("cannot ping nil indexer")
	}
	if i.Client == nil {
		return fmt.Errorf("indexer is not initialized")
	}
	resp, err := i.Client.Health()
	if err != nil {
		return err
	}
	if resp.Status != "available" {
		// See https://docs.meilisearch.com/reference/api/health.html#status
		return fmt.Errorf("status of meilisearch is not available: %s", resp.Status)
	}
	return nil
}

// Close closes the indexer
func (i *Indexer) Close() {
	if i == nil {
		return
	}
	if i.Client == nil {
		return
	}
	i.Client = nil
}
