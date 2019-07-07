// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitStatuses(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	sha1 := "1234123412341234123412341234123412341234"

	statuses, err := GetCommitStatuses(repo1, sha1, 0)
	assert.NoError(t, err)
	if assert.Len(t, statuses, 5) {
		assert.Equal(t, statuses[0].Context, "ci/awesomeness")
		assert.Equal(t, statuses[0].State, CommitStatusPending)

		assert.Equal(t, statuses[1].Context, "cov/awesomeness")
		assert.Equal(t, statuses[1].State, CommitStatusWarning)

		assert.Equal(t, statuses[2].Context, "cov/awesomeness")
		assert.Equal(t, statuses[2].State, CommitStatusSuccess)

		assert.Equal(t, statuses[3].Context, "ci/awesomeness")
		assert.Equal(t, statuses[3].State, CommitStatusFailure)

		assert.Equal(t, statuses[4].Context, "deploy/awesomeness")
		assert.Equal(t, statuses[4].State, CommitStatusError)
	}
}
