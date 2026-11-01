// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
)

func TestContentRelativePath(t *testing.T) {
	path, err := ContentRelativePath(123, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if want := "01/23/123/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.tar.gz"; path != want {
		t.Fatalf("ContentRelativePath() = %q, want %q", path, want)
	}
	for _, digest := range []string{
		"",
		"../" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef",
		"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
	} {
		if _, err := ContentRelativePath(123, digest); !errors.Is(err, ErrInvalidContentKey) {
			t.Fatalf("ContentRelativePath(%q) error = %v", digest, err)
		}
	}
	if _, err := ContentRelativePath(0, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"); !errors.Is(err, ErrInvalidContentKey) {
		t.Fatalf("zero repo ID error = %v", err)
	}
}

func TestParseContentRelativePath(t *testing.T) {
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	objectPath, err := ContentRelativePath(123, digest)
	if err != nil {
		t.Fatal(err)
	}
	repoID, gotDigest, ok := ParseContentRelativePath(objectPath)
	if !ok || repoID != 123 || gotDigest != digest {
		t.Fatalf("ParseContentRelativePath() = (%d, %q, %t)", repoID, gotDigest, ok)
	}
	for _, invalid := range []string{
		"01/23/123/not-a-digest.tar.gz",
		"01/23/0/" + digest + ".tar.gz",
		"01/23/123/" + digest[:63] + ".tar.gz",
		"01/23/123/" + digest + ".tar.gz/extra",
	} {
		if _, _, ok := ParseContentRelativePath(invalid); ok {
			t.Fatalf("ParseContentRelativePath(%q) accepted invalid path", invalid)
		}
	}
}

func TestContentStoreKeepsDigestPathImmutable(t *testing.T) {
	root := t.TempDir()
	backend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	store := &ContentStore{store: backend}
	data := []byte("checked Pages content")
	digest := sha256.Sum256(data)
	key := hex.EncodeToString(digest[:])
	for range 2 {
		if err := store.Save(17, key, bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("save attempt: %v", err)
		}
	}
	p, err := ContentRelativePath(17, key)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := bytes.Repeat([]byte("x"), len(data))
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), corrupted, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(17, key, bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("a corrupted existing digest path was overwritten")
	}
	stored, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, corrupted) {
		t.Fatalf("unexpected object after rejecting corruption: %q", stored)
	}
}
