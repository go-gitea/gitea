// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	sha1 := "1234123412341234123412341234123412341234"
	count := 4

	statuses, maxResults, err := git_model.GetCommitStatuses(db.DefaultContext, repo1, sha1, &git_model.CommitStatusOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 50}})
	assert.NoError(t, err)
	assert.Equal(t, int(maxResults), count)
	assert.Len(t, statuses, count)

	wants := []struct {
		context      string
		commitStatus structs.CommitStatusState
	}{
		{
			context:      "deploy/awesomeness",
			commitStatus: structs.CommitStatusError,
		},
		{
			context:      "ci/awesomeness",
			commitStatus: structs.CommitStatusFailure,
		},
		{
			context:      "ci/awesomeness",
			commitStatus: structs.CommitStatusPending,
		},
		{
			context:      "cov/awesomeness",
			commitStatus: structs.CommitStatusSuccess,
		},
	}
	for i := 0; i < count; i++ {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, wants[i].context, statuses[i].Context)
			assert.Equal(t, wants[i].commitStatus, statuses[i].State)
			assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[i].APIURL(db.DefaultContext))
		})
	}
}
