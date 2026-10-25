// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"

	"gitea.dev/modules/log"
	"gitea.dev/modules/storage"
)

var (
	// ErrHashMismatch occurs if the content has does not match OID
	ErrHashMismatch = errors.New("content hash does not match OID")
	// ErrSizeMismatch occurs if the content size does not match
	ErrSizeMismatch = errors.New("content size does not match")
)

// ContentStore provides a simple file system based storage.
type ContentStore struct {
	ObjectStorage storage.ObjectStorage
	TemporaryPath string
}

// NewContentStore creates the default ContentStore
func NewContentStore() *ContentStore {
	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	return contentStore
}

// Get takes a Meta object and retrieves the content from the store, returning
// it as an io.ReadSeekCloser.
func (s *ContentStore) Get(pointer Pointer) (storage.Object, error) {
	f, err := s.ObjectStorage.Open(pointer.RelativePath())
	if err != nil {
		log.Error("Whilst trying to read LFS OID[%s]: Unable to open Error: %v", pointer.Oid, err)
		return nil, err
	}
	return f, err
}

// Put takes a Meta object and an io.Reader and writes the content to the store.
func (s *ContentStore) Put(pointer Pointer, r io.Reader) (retErr error) {
	if s.TemporaryPath != "" {
		if err := os.MkdirAll(s.TemporaryPath, 0o700); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(s.TemporaryPath, "gitea-lfs-*")
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, tmp.Close(), os.Remove(tmp.Name())) }()
	wrappedRd := newHashingReader(pointer.Size, pointer.Oid, r)
	written, err := io.Copy(tmp, wrappedRd)
	if err != nil {
		return err
	}
	if written != pointer.Size {
		return ErrSizeMismatch
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	written, err = s.ObjectStorage.Save(pointer.RelativePath(), tmp, pointer.Size)
	if err != nil {
		return err
	}
	if written != pointer.Size {
		return ErrSizeMismatch
	}
	return nil
}

func (s *ContentStore) Stat(pointer Pointer) (os.FileInfo, error) {
	return s.ObjectStorage.Stat(pointer.RelativePath())
}

func (s *ContentStore) Exists(pointer Pointer) (bool, error) {
	_, err := s.Stat(pointer)
	if os.IsNotExist(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	}
	return false, err
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

// ReadMetaObject will read a git_model.LFSMetaObject and return a reader
func ReadMetaObject(pointer Pointer) (storage.Object, error) {
	contentStore := NewContentStore()
	return contentStore.Get(pointer)
}

type hashingReader struct {
	internal     io.Reader
	currentSize  int64
	expectedSize int64
	hash         hash.Hash
	expectedHash string
	lastError    error
}

// recordError records the last error during the Save operation
// Some callers of the Reader doesn't respect the returned "err"
// For example, MinIO's Put will ignore errors if the written size could equal to expected size
// So we must remember the error by ourselves,
// and later check again whether ErrSizeMismatch or ErrHashMismatch occurs during the Save operation
func (r *hashingReader) recordError(err error) error {
	r.lastError = err
	return err
}

func (r *hashingReader) Read(b []byte) (int, error) {
	if len(b) > 0 {
		remaining := r.expectedSize - r.currentSize
		if remaining < 0 {
			return 0, r.recordError(ErrSizeMismatch)
		}
		if remaining < int64(len(b)) {
			b = b[:remaining+1]
		}
	}
	n, err := r.internal.Read(b)

	if n > 0 {
		r.currentSize += int64(n)
		wn, werr := r.hash.Write(b[:n])
		if wn != n || werr != nil {
			return n, r.recordError(werr)
		}
	}

	if errors.Is(err, io.EOF) || r.currentSize >= r.expectedSize {
		if r.currentSize != r.expectedSize {
			return n, r.recordError(ErrSizeMismatch)
		}

		shaStr := hex.EncodeToString(r.hash.Sum(nil))
		if shaStr != r.expectedHash {
			return n, r.recordError(ErrHashMismatch)
		}
	}

	return n, r.recordError(err)
}

func newHashingReader(expectedSize int64, expectedHash string, reader io.Reader) *hashingReader {
	return &hashingReader{
		internal:     reader,
		expectedSize: expectedSize,
		expectedHash: expectedHash,
		hash:         sha256.New(),
	}
}
