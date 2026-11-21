// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"
	"fmt"

	"gitea.dev/models/system"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

type DBStore struct{}

func (f *DBStore) Get(ctx context.Context, item StateItem) error {
	content, err := system.GetAppStateContent(ctx, item.StateName())
	if err != nil {
		return fmt.Errorf("failed to get app state item %s: %w", item.StateName(), err)
	}
	if content == "" {
		return nil
	}
	err = json.Unmarshal(util.UnsafeStringToBytes(content), item)
	if err != nil {
		return fmt.Errorf("failed to unmarshal app state item %s: %w", item.StateName(), err)
	}
	return nil
}

func (f *DBStore) Set(ctx context.Context, item StateItem) error {
	b, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal app state item %s: %w", item.StateName(), err)
	}
	err = system.SaveAppStateContent(ctx, item.StateName(), util.UnsafeBytesToString(b))
	if err != nil {
		return fmt.Errorf("failed to save app state item %s: %w", item.StateName(), err)
	}
	return nil
}
