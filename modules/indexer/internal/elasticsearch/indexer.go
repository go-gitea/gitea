// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"sync"
	"time"

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
	stopTimer   chan struct{}
	lock        sync.RWMutex
}

func NewIndexer(client *elastic.Client, indexerName string) *Indexer {
	indexer := &Indexer{
		Client:      client,
		IndexerName: indexerName,
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
	return indexer
}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}
	ctx := graceful.GetManager().HammerContext()
	exists, err := i.Client.IndexExists(i.IndexerName).Do(ctx)
	if err != nil {
		return false, i.CheckError(err)
	}
	return exists, nil
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
