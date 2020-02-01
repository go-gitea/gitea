// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_APIFormat(t *testing.T) {
	//with HeadRepo
	assert.NoError(t, models.PrepareTestDatabase())
	pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NoError(t, pr.LoadIssue())
	apiPullRequest := ToAPIPullRequest(pr)
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head)

	//withOut HeadRepo
	pr = models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.NoError(t, pr.LoadIssue())
	assert.NoError(t, pr.LoadAttributes())
	// simulate fork deletion
	pr.HeadRepo = nil
	pr.HeadRepoID = 100000
	apiPullRequest = ToAPIPullRequest(pr)
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head.Repository)
	assert.EqualValues(t, -1, apiPullRequest.Head.RepoID)
}
