// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package meilisearch

import (
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// IndexName returns the full index name with version
func (i *Indexer) IndexName() string {
	return i.indexerName
}

func (i *Indexer) initClient() error {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   i.url,
		APIKey: i.apiKey,
	})

	i.Client = client

	i.available = true
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				i.checkAvailability()
			case <-i.stopTimer:
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func (i *Indexer) checkAvailability() {
	_, err := i.Client.Health()
	if err != nil {
		i.setAvailability(false)
		return
	}
	i.setAvailability(true)
}

func (i *Indexer) setAvailability(available bool) {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.available == available {
		return
	}

	i.available = available
}
