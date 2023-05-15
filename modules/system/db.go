// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/json"

	"github.com/yuin/goldmark/util"
)

// DBStore can be used to store app state items in local filesystem
type DBStore struct{}

// Get reads the state item
func (f *DBStore) Get(item StateItem) error {
	content, err := system.GetAppStateContent(item.Name())
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}
	return json.Unmarshal(util.StringToReadOnlyBytes(content), item)
}

// Set saves the state item
func (f *DBStore) Set(item StateItem) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return system.SaveAppStateContent(item.Name(), util.BytesToReadOnlyString(b))
}
