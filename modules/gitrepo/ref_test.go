// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestGetRefsBySha(t *testing.T) {
	storage := &mockRepository{path: "repo5_pulls"}

	// do not exist
	branches, err := GetRefsBySha(t.Context(), storage, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	assert.NoError(t, err)
	assert.Empty(t, branches)

	// refs/pull/1/head
	branches, err = GetRefsBySha(t.Context(), storage, "c83380d7056593c51a699d12b9c00627bd5743e9", git.PullPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/pull/1/head"}, branches)

	branches, err = GetRefsBySha(t.Context(), storage, "d8e0bbb45f200e67d9a784ce55bd90821af45ebd", git.BranchPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/heads/master", "refs/heads/master-clone"}, branches)

	branches, err = GetRefsBySha(t.Context(), storage, "58a4bcc53ac13e7ff76127e0fb518b5262bf09af", git.BranchPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/heads/test-patch-1"}, branches)
}

func BenchmarkGetRefsBySha(b *testing.B) {
	storage := &mockRepository{path: "repo5_pulls"}

	_, _ = GetRefsBySha(b.Context(), storage, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	_, _ = GetRefsBySha(b.Context(), storage, "d8e0bbb45f200e67d9a784ce55bd90821af45ebd", "")
	_, _ = GetRefsBySha(b.Context(), storage, "c83380d7056593c51a699d12b9c00627bd5743e9", "")
	_, _ = GetRefsBySha(b.Context(), storage, "58a4bcc53ac13e7ff76127e0fb518b5262bf09af", "")
}
