// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestGetCommitStatuses(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	sha1 := "1234123412341234123412341234123412341234"

	statuses, maxResults, err := GetCommitStatuses(repo1, sha1, &CommitStatusOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int(maxResults), 5)
	assert.Len(t, statuses, 5)

	assert.Equal(t, "ci/awesomeness", statuses[0].Context)
	assert.Equal(t, structs.CommitStatusPending, statuses[0].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[0].APIURL())

	assert.Equal(t, "cov/awesomeness", statuses[1].Context)
	assert.Equal(t, structs.CommitStatusWarning, statuses[1].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[1].APIURL())

	assert.Equal(t, "cov/awesomeness", statuses[2].Context)
	assert.Equal(t, structs.CommitStatusSuccess, statuses[2].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[2].APIURL())

	assert.Equal(t, "ci/awesomeness", statuses[3].Context)
	assert.Equal(t, structs.CommitStatusFailure, statuses[3].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[3].APIURL())

	assert.Equal(t, "deploy/awesomeness", statuses[4].Context)
	assert.Equal(t, structs.CommitStatusError, statuses[4].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[4].APIURL())
}
