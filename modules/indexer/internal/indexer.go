// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"
	"fmt"
)

// Indexer defines an basic indexer interface
type Indexer interface {
	// Init initializes the indexer
	// returns true if the index was opened/existed (with data populated), false if it was created/not-existed (with no data)
	Init(ctx context.Context) (bool, error)
	// Ping checks if the indexer is available
	Ping(ctx context.Context) error
	// Close closes the indexer
	Close()
}

// NewDummyIndexer returns a dummy indexer
func NewDummyIndexer() Indexer {
	return &dummyIndexer{}
}

type dummyIndexer struct{}

func (d *dummyIndexer) Init(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Ping(ctx context.Context) error {
	return fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Close() {}
