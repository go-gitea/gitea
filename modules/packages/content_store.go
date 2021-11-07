// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"io"
	"strconv"

	"code.gitea.io/gitea/modules/storage"
)

// ContentStore is a wrapper around ObjectStorage
type ContentStore struct {
	store storage.ObjectStorage
}

// NewContentStore creates the default package store
func NewContentStore() *ContentStore {
	contentStore := &ContentStore{storage.Packages}
	return contentStore
}

// Get gets a package blob
func (s *ContentStore) Get(packageBlobID int64) (storage.Object, error) {
	return s.store.Open(strconv.FormatInt(packageBlobID, 10))
}

// Save stores a package blob
func (s *ContentStore) Save(packageBlobID int64, r io.Reader, size int64) error {
	_, err := s.store.Save(strconv.FormatInt(packageBlobID, 10), r, size)
	return err
}

// Delete deletes a package blob
func (s *ContentStore) Delete(packageBlobID int64) error {
	return s.store.Delete(strconv.FormatInt(packageBlobID, 10))
}
