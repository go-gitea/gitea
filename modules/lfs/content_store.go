package lfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/storage"
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
	fs := storage.FileStorage{
		Ctx:      context.Background(),
		Path:     s.BasePath,
		FileName: transformKey(meta.Oid),
	}
	reader, err := fs.NewRangeReader(fromByte, -1)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// Put takes a Meta object and an io.Reader and writes the content to the store.
func (s *ContentStore) Put(meta *models.LFSMetaObject, r io.Reader) error {
	fs := storage.FileStorage{
		Ctx:      context.Background(),
		Path:     s.BasePath,
		FileName: transformKey(meta.Oid),
	}

	fw, err := fs.NewWriter()
	if err != nil {
		return err
	}

	hash := sha256.New()
	hw := io.MultiWriter(hash, fw)

	written, err := io.Copy(hw, r)
	if err != nil {
		fw.Close()
		return err
	}
	fw.Close()

	if written != meta.Size {
		return errSizeMismatch
	}

	shaStr := hex.EncodeToString(hash.Sum(nil))
	if shaStr != meta.Oid {
		return errHashMismatch
	}

	return nil
}

// Exists returns true if the object exists in the content store.
func (s *ContentStore) Exists(meta *models.LFSMetaObject) bool {
	fs := storage.FileStorage{
		Ctx:      context.Background(),
		Path:     s.BasePath,
		FileName: transformKey(meta.Oid),
	}

	return fs.Exists()
}

// Verify returns true if the object exists in the content store and size is correct.
func (s *ContentStore) Verify(meta *models.LFSMetaObject) (bool, error) {
	fs := storage.FileStorage{
		Ctx:      context.Background(),
		Path:     s.BasePath,
		FileName: transformKey(meta.Oid),
	}

	fi, err := fs.Attributes()
	if err != nil || fi.Size != meta.Size {
		return false, nil
	}

	return true, nil
}

func transformKey(key string) string {
	if len(key) < 5 {
		return key
	}

	return filepath.Join(key[0:2], key[2:4], key[4:])
}
