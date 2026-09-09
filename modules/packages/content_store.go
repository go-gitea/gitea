// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"errors"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"
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

func (s *ContentStore) OpenBlob(key BlobHash256Key) (storage.Object, error) {
	return s.store.Open(KeyToRelativePath(key))
}

func (s *ContentStore) ShouldServeDirect() bool {
	return setting.Packages.Storage.ServeDirect()
}

func (s *ContentStore) GetServeDirectURL(key BlobHash256Key, filename, method string, reqParams *storage.ServeDirectOptions) (*url.URL, error) {
	return s.store.ServeDirectURL(KeyToRelativePath(key), filename, method, reqParams)
}

func (s *ContentStore) OptionalSize(key BlobHash256Key) (sz optional.Option[int64], _ error) {
	st, err := s.store.Stat(KeyToRelativePath(key))
	if errors.Is(err, fs.ErrNotExist) {
		return sz, nil
	}
	if err != nil {
		return sz, err
	}
	return optional.Some(st.Size()), nil
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
