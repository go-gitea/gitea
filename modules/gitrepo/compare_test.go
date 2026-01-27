// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockRepository struct {
	path string
}

func (r *mockRepository) RelativePath() string {
	return r.path
}

func TestRepoGetDivergingCommits(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}
	do, err := GetDivergingCommits(t.Context(), repo, "master", "branch2")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  1,
		Behind: 5,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "master")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 0,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "test")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 2,
	}, do)
}
