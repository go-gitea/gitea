// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

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

	for _, tc := range []struct {
		branch  string
		topic   string
		missing bool
	}{
		{"non-exist-basebranch", "", true},
		{"master", "", false},
		{"master/topicbranch", "topicbranch", false},
		{"master/", "", false},
		{"/", "", true},
		{"//", "", true},
		{"master/topicbranch/", "topicbranch/", false},
		{"master/topicbranch/1", "topicbranch/1", false},
	} {
		baseBranch, currentTopicBranch, err := GetAgitBranchInfo(t.Context(), 1, tc.branch)
		if tc.missing {
			assert.ErrorIs(t, err, util.ErrNotExist)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, "master", baseBranch)
			if tc.topic == "" {
				assert.Empty(t, currentTopicBranch)
			} else {
				assert.Equal(t, tc.topic, currentTopicBranch)
			}
		}
	}
}
