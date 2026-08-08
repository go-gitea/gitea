// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !zoekt || !unix

package zoekt

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("this Gitea binary is built without the zoekt indexer, rebuild it with the \"zoekt\" build tag on a Unix system")

type Indexer struct{}

func NewIndexer() *Indexer {
	return &Indexer{}
}

func (i *Indexer) Init(_ context.Context) (bool, error) {
	return false, ErrNotImplemented
}

func (i *Indexer) Ping(_ context.Context) error {
	return ErrNotImplemented
}

func (i *Indexer) Close() {
	// NOTHING TO DO
}
