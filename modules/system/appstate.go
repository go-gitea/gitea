// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import "context"

// StateStore is the interface to get/set app state items
type StateStore interface {
	Get(ctx context.Context, item StateItem) error
	Set(ctx context.Context, item StateItem) error
}

// StateItem provides the name for a state item. the name will be used to generate filenames, etc
type StateItem interface {
	Name() string
}

// AppState contains the state items for the app
var AppState StateStore

// Init initialize AppState interface
func Init() error {
	AppState = &DBStore{}
	return nil
}
