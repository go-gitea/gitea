// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestParseAgitPushOptionValue(t *testing.T) {
	assert.Equal(t, "a", parseAgitPushOptionValue("a"))
	assert.Equal(t, "a", parseAgitPushOptionValue("{base64}YQ=="))
	assert.Equal(t, "{base64}invalid value", parseAgitPushOptionValue("{base64}invalid value"))
}

func TestGetAgitBranchInfo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, _, err := GetAgitBranchInfo(t.Context(), 1, "non-exist-basebranch")
	assert.ErrorIs(t, err, util.ErrNotExist)

	baseBranch, currentTopicBranch, err := GetAgitBranchInfo(t.Context(), 1, "master")
	assert.NoError(t, err)
	assert.Equal(t, "master", baseBranch)
	assert.Empty(t, currentTopicBranch)

	baseBranch, currentTopicBranch, err = GetAgitBranchInfo(t.Context(), 1, "master/topicbranch")
	assert.NoError(t, err)
	assert.Equal(t, "master", baseBranch)
	assert.Equal(t, "topicbranch", currentTopicBranch)

	baseBranch, currentTopicBranch, err = GetAgitBranchInfo(t.Context(), 1, "master/")
	assert.NoError(t, err)
	assert.Equal(t, "master", baseBranch)
	assert.Empty(t, currentTopicBranch)

	_, _, err = GetAgitBranchInfo(t.Context(), 1, "/")
	assert.ErrorIs(t, err, util.ErrNotExist)

	_, _, err = GetAgitBranchInfo(t.Context(), 1, "//")
	assert.ErrorIs(t, err, util.ErrNotExist)

	baseBranch, currentTopicBranch, err = GetAgitBranchInfo(t.Context(), 1, "master/topicbranch/")
	assert.NoError(t, err)
	assert.Equal(t, "master", baseBranch)
	assert.Equal(t, "topicbranch/", currentTopicBranch)

	baseBranch, currentTopicBranch, err = GetAgitBranchInfo(t.Context(), 1, "master/topicbranch/1")
	assert.NoError(t, err)
	assert.Equal(t, "master", baseBranch)
	assert.Equal(t, "topicbranch/1", currentTopicBranch)
}
