// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

var (
	errHashMismatch = errors.New("Content hash does not match OID")
	errSizeMismatch = errors.New("Content size does not match")
)

// ErrRangeNotSatisfiable represents an error which request range is not satisfiable.
type ErrRangeNotSatisfiable struct {
	FromByte int64
}

func (err ErrRangeNotSatisfiable) Error() string {
	return fmt.Sprintf("Requested range %d is not satisfiable", err.FromByte)
}

// IsErrRangeNotSatisfiable returns true if the error is an ErrRangeNotSatisfiable
func IsErrRangeNotSatisfiable(err error) bool {
	_, ok := err.(ErrRangeNotSatisfiable)
	return ok
}

// ContentStore provides a simple file system based storage.
type ContentStore struct {
	storage.ObjectStorage
}

// Get takes a Meta object and retrieves the content from the store, returning
// it as an io.Reader. If fromByte > 0, the reader starts from that byte
func (s *ContentStore) Get(meta *models.LFSMetaObject, fromByte int64) (io.ReadCloser, error) {
	f, err := s.Open(meta.RelativePath())
	if err != nil {
		log.Error("Whilst trying to read LFS OID[%s]: Unable to open Error: %v", meta.Oid, err)
		return nil, err
	}
	if fromByte > 0 {
		if fromByte >= meta.Size {
			return nil, ErrRangeNotSatisfiable{
				FromByte: fromByte,
			}
		}
		_, err = f.Seek(fromByte, io.SeekStart)
		if err != nil {
			log.Error("Whilst trying to read LFS OID[%s]: Unable to seek to %d Error: %v", meta.Oid, fromByte, err)
		}
	}
	return f, err
}

// Put takes a Meta object and an io.Reader and writes the content to the store.
func (s *ContentStore) Put(meta *models.LFSMetaObject, r io.Reader) error {
	hash := sha256.New()
	rd := io.TeeReader(r, hash)
	p := meta.RelativePath()
	written, err := s.Save(p, rd)
	if err != nil {
		log.Error("Whilst putting LFS OID[%s]: Failed to copy to tmpPath: %s Error: %v", meta.Oid, p, err)
		return err
	}

	if written != meta.Size {
		if err := s.Delete(p); err != nil {
			log.Error("Cleaning the LFS OID[%s] failed: %v", meta.Oid, err)
		}
		return errSizeMismatch
	}

	shaStr := hex.EncodeToString(hash.Sum(nil))
	if shaStr != meta.Oid {
		if err := s.Delete(p); err != nil {
			log.Error("Cleaning the LFS OID[%s] failed: %v", meta.Oid, err)
		}
		return errHashMismatch
	}

	return nil
}

// Exists returns true if the object exists in the content store.
func (s *ContentStore) Exists(meta *models.LFSMetaObject) (bool, error) {
	_, err := s.ObjectStorage.Stat(meta.RelativePath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Verify returns true if the object exists in the content store and size is correct.
func (s *ContentStore) Verify(meta *models.LFSMetaObject) (bool, error) {
	p := meta.RelativePath()
	fi, err := s.ObjectStorage.Stat(p)
	if os.IsNotExist(err) || (err == nil && fi.Size() != meta.Size) {
		return false, nil
	} else if err != nil {
		log.Error("Unable stat file: %s for LFS OID[%s] Error: %v", p, meta.Oid, err)
		return false, err
	}

	return true, nil
}
