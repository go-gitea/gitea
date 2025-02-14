// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lqinternal

import (
	"bytes"
	"encoding/binary"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func QueueItemIDBytes(id int64) []byte {
	buf := make([]byte, 8)
	binary.PutVarint(buf, id)
	return buf
}

func QueueItemKeyBytes(prefix []byte, id int64) []byte {
	key := make([]byte, len(prefix), len(prefix)+1+8)
	copy(key, prefix)
	key = append(key, '-')
	return append(key, QueueItemIDBytes(id)...)
}

func RemoveLevelQueueKeys(db *leveldb.DB, namePrefix []byte) {
	keyPrefix := make([]byte, len(namePrefix)+1)
	copy(keyPrefix, namePrefix)
	keyPrefix[len(namePrefix)] = '-'

	it := db.NewIterator(nil, &opt.ReadOptions{Strict: opt.NoStrict})
	defer it.Release()
	for it.Next() {
		if bytes.HasPrefix(it.Key(), keyPrefix) {
			_ = db.Delete(it.Key(), nil)
		}
	}
}

func ListLevelQueueKeys(db *leveldb.DB) (res [][]byte) {
	it := db.NewIterator(nil, &opt.ReadOptions{Strict: opt.NoStrict})
	defer it.Release()
	for it.Next() {
		res = append(res, it.Key())
	}
	return res
}
