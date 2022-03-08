// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

var (
	// ErrHashMismatch occurs if the content has does not match OID
	ErrHashMismatch = errors.New("Content hash does not match OID")
	// ErrSizeMismatch occurs if the content size does not match
	ErrSizeMismatch = errors.New("Content size does not match")
)

// ErrRangeNotSatisfiable represents an error which request range is not satisfiable.
type ErrRangeNotSatisfiable struct {
	FromByte int64
}

// IsErrRangeNotSatisfiable returns true if the error is an ErrRangeNotSatisfiable
func IsErrRangeNotSatisfiable(err error) bool {
	_, ok := err.(ErrRangeNotSatisfiable)
	return ok
}

func (err ErrRangeNotSatisfiable) Error() string {
	return fmt.Sprintf("Requested range %d is not satisfiable", err.FromByte)
}

// ContentStore provides a simple file system based storage.
type ContentStore struct {
	storage.ObjectStorage
}

// NewContentStore creates the default ContentStore
func NewContentStore() *ContentStore {
	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	return contentStore
}

// Get takes a Meta object and retrieves the content from the store, returning
// it as an io.ReadSeekCloser.
func (s *ContentStore) Get(pointer Pointer) (storage.Object, error) {
	f, err := s.Open(pointer.RelativePath())
	if err != nil {
		log.Error("Whilst trying to read LFS OID[%s]: Unable to open Error: %v", pointer.Oid, err)
		return nil, err
	}
	return f, err
}

// Put takes a Meta object and an io.Reader and writes the content to the store.
func (s *ContentStore) Put(pointer Pointer, r io.Reader) error {
	p := pointer.RelativePath()

	// Wrap the provided reader with an inline hashing and size checker
	wrappedRd := newHashingReader(pointer.Size, pointer.Oid, r)

	// now pass the wrapped reader to Save - if there is a size mismatch or hash mismatch then
	// the errors returned by the newHashingReader should percolate up to here
	written, err := s.Save(p, wrappedRd, pointer.Size)
	if err != nil {
		log.Error("Whilst putting LFS OID[%s]: Failed to copy to tmpPath: %s Error: %v", pointer.Oid, p, err)
		return err
	}

	// This shouldn't happen but it is sensible to test
	if written != pointer.Size {
		if err := s.Delete(p); err != nil {
			log.Error("Cleaning the LFS OID[%s] failed: %v", pointer.Oid, err)
		}
		return ErrSizeMismatch
	}

	return nil
}

// Exists returns true if the object exists in the content store.
func (s *ContentStore) Exists(pointer Pointer) (bool, error) {
	_, err := s.ObjectStorage.Stat(pointer.RelativePath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Verify returns true if the object exists in the content store and size is correct.
func (s *ContentStore) Verify(pointer Pointer) (bool, error) {
	p := pointer.RelativePath()
	fi, err := s.ObjectStorage.Stat(p)
	if os.IsNotExist(err) || (err == nil && fi.Size() != pointer.Size) {
		return false, nil
	} else if err != nil {
		log.Error("Unable stat file: %s for LFS OID[%s] Error: %v", p, pointer.Oid, err)
		return false, err
	}

	return true, nil
}

// ReadMetaObject will read a models.LFSMetaObject and return a reader
func ReadMetaObject(pointer Pointer) (io.ReadCloser, error) {
	contentStore := NewContentStore()
	return contentStore.Get(pointer)
}

type hashingReader struct {
	internal     io.Reader
	currentSize  int64
	expectedSize int64
	hash         hash.Hash
	expectedHash string
}

func (r *hashingReader) Read(b []byte) (int, error) {
	n, err := r.internal.Read(b)

	if n > 0 {
		r.currentSize += int64(n)
		wn, werr := r.hash.Write(b[:n])
		if wn != n || werr != nil {
			return n, werr
		}
	}

	if err != nil && err == io.EOF {
		if r.currentSize != r.expectedSize {
			return n, ErrSizeMismatch
		}

		shaStr := hex.EncodeToString(r.hash.Sum(nil))
		if shaStr != r.expectedHash {
			return n, ErrHashMismatch
		}
	}

	return n, err
}

func newHashingReader(expectedSize int64, expectedHash string, reader io.Reader) *hashingReader {
	return &hashingReader{
		internal:     reader,
		expectedSize: expectedSize,
		expectedHash: expectedHash,
		hash:         sha256.New(),
	}
}
