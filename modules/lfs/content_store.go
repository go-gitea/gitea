package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models"
)

var (
	errHashMismatch = errors.New("Content hash does not match OID")
	errSizeMismatch = errors.New("Content size does not match")
)

// ContentStore provides a simple file system based storage.
type ContentStore struct {
	BasePath string
}

// Get takes a Meta object and retrieves the content from the store, returning
// it as an io.Reader. If fromByte > 0, the reader starts from that byte
func (s *ContentStore) Get(meta *models.LFSMetaObject, fromByte int64) (io.ReadCloser, error) {
	path := filepath.Join(s.BasePath, transformKey(meta.Oid))

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
func (s *ContentStore) Put(meta *models.LFSMetaObject, r io.Reader) error {
	path := filepath.Join(s.BasePath, transformKey(meta.Oid))
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

	return os.Rename(tmpPath, path)
}

// Exists returns true if the object exists in the content store.
func (s *ContentStore) Exists(meta *models.LFSMetaObject) bool {
	path := filepath.Join(s.BasePath, transformKey(meta.Oid))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// Verify returns true if the object exists in the content store and size is correct.
func (s *ContentStore) Verify(meta *models.LFSMetaObject) (bool, error) {
	path := filepath.Join(s.BasePath, transformKey(meta.Oid))

	fi, err := os.Stat(path)
	if os.IsNotExist(err) || err == nil && fi.Size() != meta.Size {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func transformKey(key string) string {
	if len(key) < 5 {
		return key
	}

	return filepath.Join(key[0:2], key[2:4], key[4:])
}
