// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"sync"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/internal"

	"github.com/olivere/elastic/v7"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic elasticsearch indexer implementation
type Indexer struct {
	Client      *elastic.Client
	IndexerName string
	available   bool
	StopTimer   chan struct{}
	lock        sync.RWMutex
}

func NewIndexer(client *elastic.Client, indexerName string) *Indexer {
	return &Indexer{
		Client:      client,
		IndexerName: indexerName,
		available:   true,
		StopTimer:   make(chan struct{}),
	}
}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	ctx := graceful.GetManager().HammerContext()
	exists, err := i.Client.IndexExists(i.IndexerName).Do(ctx)
	if err != nil {
		return false, i.CheckError(err)
	}
	return exists, nil
}

// Ping checks if the indexer is available
func (i *Indexer) Ping() bool {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.available
}

// Close closes the indexer
func (i *Indexer) Close() {
	select {
	case <-i.StopTimer:
	default:
		close(i.StopTimer)
	}
}
