// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

type GPGKeyList []*GPGKey

func (keys GPGKeyList) keyIDs() []string {
	ids := make([]string, len(keys))
	for i, key := range keys {
		ids[i] = key.KeyID
	}
	return ids
}

func (keys GPGKeyList) LoadSubKeys(ctx context.Context) error {
	subKeys := make([]*GPGKey, 0, len(keys))
	if err := db.GetEngine(ctx).In("primary_key_id", keys.keyIDs()).Find(&subKeys); err != nil {
		return err
	}
	subKeysMap := make(map[string][]*GPGKey, len(subKeys))
	for _, key := range subKeys {
		subKeysMap[key.PrimaryKeyID] = append(subKeysMap[key.PrimaryKeyID], key)
	}

	for _, key := range keys {
		if subKeys, ok := subKeysMap[key.KeyID]; ok {
			key.SubsKey = subKeys
		}
	}
	return nil
}
