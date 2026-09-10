// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"bytes"
	"testing"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddLFSMirrorPending_SharedOIDAcrossRepos ensures two different repositories
// can both have a pending row for the same OID. The unique constraint must be
// composite (repository_id, oid), not global on oid alone.
func TestAddLFSMirrorPending_SharedOIDAcrossRepos(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pointer, err := lfs.GeneratePointer(bytes.NewReader([]byte("shared lfs content")))
	require.NoError(t, err)
	pointerBlob := lfs.PointerBlob{Hash: "abc123", Pointer: pointer}

	added, err := git_model.AddLFSMirrorPending(t.Context(), 1, pointerBlob)
	require.NoError(t, err)
	assert.True(t, added)

	// Same OID, different repo — must not hit a unique constraint violation.
	added, err = git_model.AddLFSMirrorPending(t.Context(), 2, pointerBlob)
	require.NoError(t, err)
	assert.True(t, added)
}

// TestAddLFSMirrorPending_Idempotent ensures inserting the same (repo, oid) twice
// is a no-op and does not return an error.
func TestAddLFSMirrorPending_Idempotent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pointer, err := lfs.GeneratePointer(bytes.NewReader([]byte("idempotent content")))
	require.NoError(t, err)
	pointerBlob := lfs.PointerBlob{Hash: "abc123", Pointer: pointer}

	added, err := git_model.AddLFSMirrorPending(t.Context(), 1, pointerBlob)
	require.NoError(t, err)
	assert.True(t, added)

	added, err = git_model.AddLFSMirrorPending(t.Context(), 1, pointerBlob)
	require.NoError(t, err)
	assert.False(t, added)
}
