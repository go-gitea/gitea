// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
