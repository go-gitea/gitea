// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"fmt"
	"sync"

	"github.com/meilisearch/meilisearch-go"
)

// Indexer represents a basic meilisearch indexer implementation
type Indexer struct {
	Client *meilisearch.Client

	url, apiKey string
	indexerName string
	available   bool
	stopTimer   chan struct{}
	lock        sync.RWMutex
}

func NewIndexer(url, apiKey, indexerName string) *Indexer {
	return &Indexer{
		url:         url,
		apiKey:      apiKey,
		indexerName: indexerName,
		available:   false,
		stopTimer:   make(chan struct{}),
	}
}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}

	if err := i.initClient(); err != nil {
		return false, err
	}
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
func (i *Indexer) Ping() bool {
	if i == nil {
		return false
	}
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.available
}

// Close closes the indexer
func (i *Indexer) Close() {
	if i == nil {
		return
	}
	select {
	case <-i.stopTimer:
	default:
		close(i.stopTimer)
	}
}
