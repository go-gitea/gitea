// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_APIFormat(t *testing.T) {
	// with HeadRepo
	assert.NoError(t, unittest.PrepareTestDatabase())
	headRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadAttributes(t.Context()))
	assert.NoError(t, pr.LoadIssue(t.Context()))
	apiPullRequest := ToAPIPullRequest(t.Context(), pr, nil)
	assert.NotNil(t, apiPullRequest)
	assert.Equal(t, &structs.PRBranchInfo{
		Name:       "branch1",
		Ref:        "refs/pull/2/head",
		Sha:        "4a357436d925b5c974181ff12a994538ddc5a269",
		RepoID:     1,
		Repository: ToRepo(t.Context(), headRepo, access_model.Permission{AccessMode: perm.AccessModeRead}),
	}, apiPullRequest.Head)

	// withOut HeadRepo
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadAttributes(t.Context()))
	// simulate fork deletion
	pr.HeadRepo = nil
	pr.HeadRepoID = 100000
	apiPullRequest = ToAPIPullRequest(t.Context(), pr, nil)
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head.Repository)
	assert.EqualValues(t, -1, apiPullRequest.Head.RepoID)

	apiPullRequests, err := ToAPIPullRequests(t.Context(), pr.BaseRepo, []*issues_model.PullRequest{pr}, nil)
	assert.NoError(t, err)
	assert.Len(t, apiPullRequests, 1)
	assert.NotNil(t, apiPullRequests[0])
	assert.Nil(t, apiPullRequests[0].Head.Repository)
	assert.EqualValues(t, -1, apiPullRequests[0].Head.RepoID)
}
