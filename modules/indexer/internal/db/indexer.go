// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"code.gitea.io/gitea/modules/indexer/internal"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic db indexer implementation
type Indexer struct{}

// Init initializes the indexer
func (i *Indexer) Init(_ context.Context) (bool, error) {
	// Return true to indicate that the index was opened/existed.
	// So that the indexer will not try to populate the index, the data is already there.
	return true, nil
}

// Ping checks if the indexer is available
func (i *Indexer) Ping(_ context.Context) error {
	// No need to ping database to check if it is available.
	// If the database goes down, Gitea will go down, so nobody will care if the indexer is available.
	return nil
}

// Close closes the indexer
func (i *Indexer) Close() {
	// nothing to do
}
