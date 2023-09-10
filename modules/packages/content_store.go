// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

// BlobHash256Key is the key to address a blob content
type BlobHash256Key string

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
func (s *ContentStore) Get(key BlobHash256Key) (storage.Object, error) {
	return s.store.Open(KeyToRelativePath(key))
}

func (s *ContentStore) ShouldServeDirect() bool {
	return setting.Packages.Storage.MinioConfig.ServeDirect
}

func (s *ContentStore) GetServeDirectURL(key BlobHash256Key, filename string) (*url.URL, error) {
	return s.store.URL(KeyToRelativePath(key), filename)
}

// FIXME: Workaround to be removed in v1.20
// https://github.com/go-gitea/gitea/issues/19586
func (s *ContentStore) Has(key BlobHash256Key) error {
	_, err := s.store.Stat(KeyToRelativePath(key))
	return err
}

// Save stores a package blob
func (s *ContentStore) Save(key BlobHash256Key, r io.Reader, size int64) error {
	_, err := s.store.Save(KeyToRelativePath(key), r, size)
	return err
}

// Delete deletes a package blob
func (s *ContentStore) Delete(key BlobHash256Key) error {
	return s.store.Delete(KeyToRelativePath(key))
}

// KeyToRelativePath converts the sha256 key aabb000000... to aa/bb/aabb000000...
func KeyToRelativePath(key BlobHash256Key) string {
	return path.Join(string(key)[0:2], string(key)[2:4], string(key))
}

// RelativePathToKey converts a relative path aa/bb/aabb000000... to the sha256 key aabb000000...
func RelativePathToKey(relativePath string) (BlobHash256Key, error) {
	parts := strings.SplitN(relativePath, "/", 3)
	if len(parts) != 3 || len(parts[0]) != 2 || len(parts[1]) != 2 || len(parts[2]) < 4 || parts[0]+parts[1] != parts[2][0:4] {
		return "", util.ErrInvalidArgument
	}

	return BlobHash256Key(parts[2]), nil
}
