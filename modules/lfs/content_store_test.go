// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/storage"

	"github.com/stretchr/testify/require"
)

type unreadStorage struct{ storage.ObjectStorage }

func (unreadStorage) Save(_path string, _reader io.Reader, _size int64) (int64, error) {
	return 0, nil
}

func TestContentStorePutZeroSizeVerifiesUnreadStream(t *testing.T) {
	store := &ContentStore{ObjectStorage: unreadStorage{}}
	empty, err := GeneratePointer(strings.NewReader(""))
	require.NoError(t, err)
	require.NoError(t, store.Put(empty, strings.NewReader("")))
	require.ErrorIs(t, store.Put(empty, strings.NewReader("x")), ErrSizeMismatch)
	require.ErrorIs(t, store.Put(Pointer{Oid: strings.Repeat("0", 64)}, strings.NewReader("")), ErrHashMismatch)
}
