package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var (
	errHashMismatch = errors.New("Content hash does not match OID")
	errSizeMismatch = errors.New("Content size does not match")
)

// ContentStore provides a simple file system based storage.
type ContentStore struct {
	basePath string
}

// NewContentStore creates a ContentStore at the base directory.
func NewContentStore(base string) (*ContentStore, error) {
	if err := os.MkdirAll(base, 0750); err != nil {
		return nil, err
	}

	return &ContentStore{base}, nil
}

// Get takes a Meta object and retreives the content from the store, returning
// it as an io.Reader. If fromByte > 0, the reader starts from that byte
func (s *ContentStore) Get(meta *MetaObject, fromByte int64) (io.Reader, error) {
	path := filepath.Join(s.basePath, transformKey(meta.Oid))

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if fromByte > 0 {
		_, err = f.Seek(fromByte, os.SEEK_CUR)
	}
	return f, err
}

// Put takes a Meta object and an io.Reader and writes the content to the store.
func (s *ContentStore) Put(meta *MetaObject, r io.Reader) error {
	path := filepath.Join(s.basePath, transformKey(meta.Oid))
	tmpPath := path + ".tmp"

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	hash := sha256.New()
	hw := io.MultiWriter(hash, file)

	written, err := io.Copy(hw, r)
	if err != nil {
		file.Close()
		return err
	}
	file.Close()

	if written != meta.Size {
		return errSizeMismatch
	}

	shaStr := hex.EncodeToString(hash.Sum(nil))
	if shaStr != meta.Oid {
		return errHashMismatch
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

// Exists returns true if the object exists in the content store.
func (s *ContentStore) Exists(meta *MetaObject) bool {
	path := filepath.Join(s.basePath, transformKey(meta.Oid))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func transformKey(key string) string {
	if len(key) < 5 {
		return key
	}

	return filepath.Join(key[0:2], key[2:4], key[4:len(key)])
}
