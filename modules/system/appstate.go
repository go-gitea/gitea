// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import "context"

// StateStore is the interface to get/set app state items
type StateStore interface {
	Get(ctx context.Context, item StateItem) error
	Set(ctx context.Context, item StateItem) error
}

// StateItem provides the name for a state item that is used to the generate key for persistence
type StateItem interface {
	StateName() string
}

var AppState StateStore

// Init initialize AppState interface to use database to store app state items
func Init() error {
	AppState = &DBStore{}
	return nil
}
