// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/json"

	"github.com/yuin/goldmark/util"
)

// DBStore can be used to store app state items in local filesystem
type DBStore struct{}

// Get reads the state item
func (f *DBStore) Get(ctx context.Context, item StateItem) error {
	content, err := system.GetAppStateContent(ctx, item.Name())
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}
	return json.Unmarshal(util.StringToReadOnlyBytes(content), item)
}

// Set saves the state item
func (f *DBStore) Set(ctx context.Context, item StateItem) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return system.SaveAppStateContent(ctx, item.Name(), util.BytesToReadOnlyString(b))
}
