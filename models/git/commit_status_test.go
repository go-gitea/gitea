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

	statuses, maxResults, err := git_model.GetCommitStatuses(db.DefaultContext, repo1, sha1, &git_model.CommitStatusOptions{ListOptions: db.ListOptions{Page: 1, PageSize: 50}})
	assert.NoError(t, err)
	assert.Equal(t, int(maxResults), 5)
	assert.Len(t, statuses, 5)

	assert.Equal(t, "ci/awesomeness", statuses[0].Context)
	assert.Equal(t, structs.CommitStatusPending, statuses[0].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[0].APIURL(db.DefaultContext))

	assert.Equal(t, "cov/awesomeness", statuses[1].Context)
	assert.Equal(t, structs.CommitStatusWarning, statuses[1].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[1].APIURL(db.DefaultContext))

	assert.Equal(t, "cov/awesomeness", statuses[2].Context)
	assert.Equal(t, structs.CommitStatusSuccess, statuses[2].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[2].APIURL(db.DefaultContext))

	assert.Equal(t, "ci/awesomeness", statuses[3].Context)
	assert.Equal(t, structs.CommitStatusFailure, statuses[3].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[3].APIURL(db.DefaultContext))

	assert.Equal(t, "deploy/awesomeness", statuses[4].Context)
	assert.Equal(t, structs.CommitStatusError, statuses[4].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[4].APIURL(db.DefaultContext))
}
