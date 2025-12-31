// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetTreePathLatestCommit(t *testing.T) {
	storage := &mockRepository{path: "repo6_blame"}

	commitID, err := GetTreePathLatestCommitID(t.Context(), storage, "master", "blame.txt")
	assert.NoError(t, err)
	assert.Equal(t, "45fb6cbc12f970b04eacd5cd4165edd11c8d7376", commitID)
}
