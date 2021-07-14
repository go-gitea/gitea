// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"fmt"
	"io"

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

// Get get the package file content
func (s *ContentStore) Get(packageID, packageFileID int64) (storage.Object, error) {
	return s.store.Open(toRelativePath(packageID, packageFileID))
}

// Save stores the package file content
func (s *ContentStore) Save(packageID, packageFileID int64, r io.Reader, size int64) error {
	_, err := s.store.Save(toRelativePath(packageID, packageFileID), r, size)
	return err
}

// Delete deletes the package file content
func (s *ContentStore) Delete(packageID, packageFileID int64) error {
	return s.store.Delete(toRelativePath(packageID, packageFileID))
}

func toRelativePath(packageID, packageFileID int64) string {
	return fmt.Sprintf("%d/%d", packageID, packageFileID)
}
