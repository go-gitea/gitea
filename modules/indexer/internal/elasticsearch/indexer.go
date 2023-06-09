// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/internal"

	"github.com/olivere/elastic/v7"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic elasticsearch indexer implementation
type Indexer struct {
	Client *elastic.Client

	url            string
	indexAliasName string
	version        int
	mapping        string

	available bool
	stopTimer chan struct{}
	lock      sync.RWMutex
}

func NewIndexer(url, indexName string, version int, mapping string) *Indexer {
	return &Indexer{
		url:            url,
		indexAliasName: indexName,
		version:        version,
		mapping:        mapping,
		available:      false,
		stopTimer:      make(chan struct{}),
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

	ctx := graceful.GetManager().HammerContext()

	exists, err := i.Client.IndexExists(i.IndexName()).Do(ctx)
	if err != nil {
		return false, i.CheckError(err)
	}
	if exists {
		return true, nil
	}

	if err := i.createIndex(ctx); err != nil {
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
