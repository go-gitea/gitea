// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lstree

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/git"

	bolt "go.etcd.io/bbolt"
)

var (
	_ git.LsTreeCache = &BoltDBCache{}
)

// BoltDBCache implements git.LsTreeCache interface to save the git tree entries on boltdb
type BoltDBCache struct {
	cacheDir string
	bucket   []byte
	db       *bolt.DB
}

// NewBoltDBCache creates a boltdb cache
func NewBoltDBCache(cacheDir string) (*BoltDBCache, error) {
	db, err := bolt.Open(cacheDir, 0600, nil)
	if err != nil {
		return nil, err
	}

	var bucket = []byte("default")
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &BoltDBCache{
		cacheDir: cacheDir,
		bucket:   bucket,
		db:       db,
	}, nil
}

// Get implements git.LsTreeCache
func (c *BoltDBCache) Get(repoPath, treeIsh string) (git.Entries, error) {
	var entries git.Entries
	var found bool
	err := c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.bucket)
		v := b.Get([]byte(getKey(repoPath, treeIsh)))
		if v == nil || len(v) <= 0 {
			return nil
		}
		found = true
		return json.Unmarshal(v, &entries)
	})
	if err != nil {
		return nil, err
	}
	if found {
		return entries, nil
	}
	return nil, nil
}

// Put implements git.LsTreeCache
func (c *BoltDBCache) Put(repoPath, treeIsh string, entries git.Entries) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.bucket)
		v, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		return b.Put([]byte(getKey(repoPath, treeIsh)), v)
	})
	return err
}
