// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestGetMergeablePullRequestsByHeadCommitID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pulls, err := pull.GetMergeablePullRequestsByHeadCommitID(t.Context(), repo1, "985f0301dba5e7b34be866819cd15ad3d8f508ee")
	assert.NoError(t, err)
	assert.Len(t, pulls, 1)
}
