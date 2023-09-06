// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/indexer/internal"

	"github.com/olivere/elastic/v7"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic elasticsearch indexer implementation
type Indexer struct {
	Client *elastic.Client

	url       string
	indexName string
	version   int
	mapping   string
}

func NewIndexer(url, indexName string, version int, mapping string) *Indexer {
	return &Indexer{
		url:       url,
		indexName: indexName,
		version:   version,
		mapping:   mapping,
	}
}

// Init initializes the indexer
func (i *Indexer) Init(ctx context.Context) (bool, error) {
	if i == nil {
		return false, fmt.Errorf("cannot init nil indexer")
	}
	if i.Client != nil {
		return false, fmt.Errorf("indexer is already initialized")
	}

	client, err := i.initClient()
	if err != nil {
		return false, err
	}
	i.Client = client

	exists, err := i.Client.IndexExists(i.VersionedIndexName()).Do(ctx)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	if err := i.createIndex(ctx); err != nil {
		return false, err
	}

	return exists, nil
}

// Ping checks if the indexer is available
func (i *Indexer) Ping(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("cannot ping nil indexer")
	}
	if i.Client == nil {
		return fmt.Errorf("indexer is not initialized")
	}

	resp, err := i.Client.ClusterHealth().Do(ctx)
	if err != nil {
		return err
	}
	if resp.Status != "green" && resp.Status != "yellow" {
		// It's healthy if the status is green, and it's available if the status is yellow,
		// see https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html
		return fmt.Errorf("status of elasticsearch cluster is %s", resp.Status)
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
