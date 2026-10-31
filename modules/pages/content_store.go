// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"gitea.dev/modules/storage"
)

// ErrInvalidContentKey reports an invalid repository ID or digest.
var ErrInvalidContentKey = errors.New("invalid Pages content key")

// ContentRelativePath returns the stable object path for a repository and content digest.
func ContentRelativePath(repoID int64, digest string) (string, error) {
	if repoID <= 0 || len(digest) != 64 || digest != strings.ToLower(digest) {
		return "", ErrInvalidContentKey
	}
	var decoded [32]byte
	if _, err := hex.Decode(decoded[:], []byte(digest)); err != nil {
		return "", ErrInvalidContentKey
	}
	return path.Join(digest[:2], digest[2:4], strconv.FormatInt(repoID, 10), digest+".tar.gz"), nil
}

// ParseContentRelativePath validates and decodes an object path produced by
// ContentRelativePath. Storage maintenance uses it to relate an object to the one
// current digest recorded for its repository without trusting arbitrary path
// components as database identifiers.
func ParseContentRelativePath(objectPath string) (repoID int64, digest string, ok bool) {
	clean := path.Clean(strings.TrimPrefix(objectPath, "/"))
	parts := strings.Split(clean, "/")
	if len(parts) != 4 || len(parts[0]) != 2 || len(parts[1]) != 2 || !strings.HasSuffix(parts[3], ".tar.gz") {
		return 0, "", false
	}
	digest = strings.TrimSuffix(parts[3], ".tar.gz")
	if len(digest) != 64 || digest != strings.ToLower(digest) {
		return 0, "", false
	}
	var decoded [sha256.Size]byte
	if _, err := hex.Decode(decoded[:], []byte(digest)); err != nil {
		return 0, "", false
	}
	var err error
	repoID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || repoID <= 0 {
		return 0, "", false
	}
	canonical, err := ContentRelativePath(repoID, digest)
	if err != nil || canonical != clean {
		return 0, "", false
	}
	return repoID, digest, true
}

// ContentStore provides Pages-specific object paths over a Gitea storage backend.
type ContentStore struct{ store storage.ObjectStorage }

// NewContentStore creates a store over the configured Pages backend.
func NewContentStore() *ContentStore { return &ContentStore{store: storage.Pages} }

func (s *ContentStore) relativePath(repoID int64, digest string) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("ContentStore.relativePath: %w: nil storage backend", ErrInvalidContentKey)
	}
	return ContentRelativePath(repoID, digest)
}

// Save writes an object at its digest-derived path.
func (s *ContentStore) Save(repoID int64, digest string, r io.Reader, size int64) error {
	p, err := s.relativePath(repoID, digest)
	if err != nil {
		return err
	}
	// A digest path is immutable. Reusing a good object avoids a backend
	// overwrite; a bad object must be repaired explicitly by the operator.
	info, err := s.store.Stat(p)
	if err == nil {
		if info.Size() != size {
			return fmt.Errorf("ContentStore.Save: Pages object %s has the wrong size", p)
		}
		object, err := s.store.Open(p)
		if err != nil {
			return fmt.Errorf("ContentStore.Save: open existing Pages object: %w", err)
		}
		h := sha256.New()
		read, hashErr := io.Copy(h, object)
		closeErr := object.Close()
		if hashErr != nil || closeErr != nil {
			return fmt.Errorf("ContentStore.Save: read existing Pages object: %w", errors.Join(hashErr, closeErr))
		}
		if read != size || hex.EncodeToString(h.Sum(nil)) != digest {
			return fmt.Errorf("ContentStore.Save: Pages object %s does not match its digest", p)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ContentStore.Save: stat Pages object: %w", err)
	}
	h := sha256.New()
	written, err := s.store.Save(p, io.TeeReader(r, h), size)
	if err != nil {
		return err
	}
	if written != size || hex.EncodeToString(h.Sum(nil)) != digest {
		_ = s.store.Delete(p)
		return fmt.Errorf("ContentStore.Save: saved Pages object %s does not match its digest or size", p)
	}
	return nil
}

// Open opens an object at its digest-derived path.
func (s *ContentStore) Open(repoID int64, digest string) (storage.Object, error) {
	p, err := s.relativePath(repoID, digest)
	if err != nil {
		return nil, err
	}
	return s.store.Open(p)
}

// Stat obtains metadata for an object at its digest-derived path.
func (s *ContentStore) Stat(repoID int64, digest string) (os.FileInfo, error) {
	p, err := s.relativePath(repoID, digest)
	if err != nil {
		return nil, err
	}
	return s.store.Stat(p)
}

// Delete removes an object at its digest-derived path.
func (s *ContentStore) Delete(repoID int64, digest string) error {
	p, err := s.relativePath(repoID, digest)
	if err != nil {
		return err
	}
	return s.store.Delete(p)
}
