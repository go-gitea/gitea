// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"context"
	"errors"
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

// Indexer represents a basic meilisearch indexer implementation
type Indexer struct {
	Client meilisearch.ServiceManager

	url, apiKey string
	indexName   string
	version     int
	settings    *meilisearch.Settings
}

func NewIndexer(url, apiKey, indexName string, version int, settings *meilisearch.Settings) *Indexer {
	return &Indexer{
		url:       url,
		apiKey:    apiKey,
		indexName: indexName,
		version:   version,
		settings:  settings,
	}
}

// Init initializes the indexer
func (i *Indexer) Init(_ context.Context) (bool, error) {
	if i == nil {
		return false, errors.New("cannot init nil indexer")
	}

	if i.Client != nil {
		return false, errors.New("indexer is already initialized")
	}

	i.Client = meilisearch.New(i.url, meilisearch.WithAPIKey(i.apiKey))
	_, err := i.Client.GetIndex(i.VersionedIndexName())
	if err == nil {
		return true, nil
	}
	_, err = i.Client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        i.VersionedIndexName(),
		PrimaryKey: "id",
	})
	if err != nil {
		return false, err
	}

	i.checkOldIndexes()

	_, err = i.Client.Index(i.VersionedIndexName()).UpdateSettings(i.settings)
	return false, err
}

// Ping checks if the indexer is available
func (i *Indexer) Ping(ctx context.Context) error {
	if i == nil {
		return errors.New("cannot ping nil indexer")
	}
	if i.Client == nil {
		return errors.New("indexer is not initialized")
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
	i.Client = nil
}
