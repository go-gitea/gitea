// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

// Queue defines an interface to save an issue indexer queue
type Queue interface {
	Run() error
	Push(*IndexerData) error
}

// DummyQueue represents an empty queue
type DummyQueue struct {
}

// Run starts to run the queue
func (b *DummyQueue) Run() error {
	return nil
}

// Push pushes data to indexer
func (b *DummyQueue) Push(*IndexerData) error {
	return nil
}

// NoQueue represents a direct index
type NoQueue struct {
	indexer Indexer
}

// NewNoQueue creates a noqueue object
func NewNoQueue(indexer Indexer) *NoQueue {
	return &NoQueue{
		indexer: indexer,
	}
}

// Run starts to run the queue
func (b *NoQueue) Run() error {
	return nil
}

// Push pushes data to indexer
func (b *NoQueue) Push(data *IndexerData) error {
	if data.IsDelete {
		return b.indexer.Delete(data.IDs...)
	}

	return b.indexer.Index([]*IndexerData{data})
}
