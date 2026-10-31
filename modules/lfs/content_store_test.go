// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stagingReader struct {
	io.Reader
	check func()
}

func (r stagingReader) Read(p []byte) (int, error) {
	r.check()
	return r.Reader.Read(p)
}

func TestContentStoreStaging(t *testing.T) {
	for _, name := range []string{"local", "local-custom", "remote", "override"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			config := &setting.Storage{Path: filepath.Join(root, "objects")}
			if name == "local-custom" {
				config.TemporaryPath = filepath.Join(root, "storage-tmp")
			}
			backend, err := storage.NewLocalStorage(t.Context(), config)
			require.NoError(t, err)
			store := &ContentStore{ObjectStorage: backend}
			expectedPath := filepath.Join(config.Path, "tmp")
			if config.TemporaryPath != "" {
				expectedPath = config.TemporaryPath
			}
			defer test.MockVariableValue(&setting.AppDataPath, filepath.Join(root, "app-data"))()
			if name == "remote" {
				store.ObjectStorage = struct{ storage.ObjectStorage }{backend}
				expectedPath = setting.AppDataTempDir("lfs").JoinPath()
			} else if name == "override" {
				store.TemporaryPath = filepath.Join(root, "operation-tmp")
				expectedPath = store.TemporaryPath
			}
			for _, env := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(env, filepath.Join(root, "unavailable"))
			}
			data := []byte("verified upload")
			pointer, err := GeneratePointer(bytes.NewReader(data))
			require.NoError(t, err)
			r := stagingReader{Reader: bytes.NewReader(data), check: func() {
				files, err := filepath.Glob(filepath.Join(expectedPath, "gitea-lfs-*"))
				require.NoError(t, err)
				require.Len(t, files, 1)
			}}
			require.NoError(t, store.Put(pointer, r))
			entries, err := os.ReadDir(expectedPath)
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	}
}

func TestContentStoreRejectsInvalidUpload(t *testing.T) {
	backend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	store := &ContentStore{ObjectStorage: backend}
	valid := []byte("verified object")
	p := Pointer{Oid: fmt.Sprintf("%x", sha256.Sum256(valid)), Size: int64(len(valid))}
	require.NoError(t, store.Put(p, bytes.NewReader(valid)))
	require.ErrorIs(t, store.Put(p, bytes.NewReader(bytes.Repeat([]byte("x"), len(valid)))), ErrHashMismatch)
	oversized := bytes.NewReader(bytes.Repeat([]byte("x"), 64*1024))
	require.ErrorIs(t, store.Put(p, oversized), ErrSizeMismatch)
	assert.Equal(t, p.Size+1, oversized.Size()-int64(oversized.Len()))
	object, err := store.Get(p)
	require.NoError(t, err)
	defer object.Close()
	data, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, valid, data)
}
