// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPIPullCommits(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 6}).(*models.PullRequest)
	assert.NoError(t, pullIssue.LoadIssue())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.HeadRepoID}).(*models.Repository)

	// test ListPullReviews
	session := loginUser(t, "user2")
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits", repo.OwnerName, repo.Name, pullIssue.Index)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var commits []*api.Commit
	DecodeJSON(t, resp, &commits)
	if !assert.Len(t, commits, 3) {
		return
	}
	assert.Equal(t, "c711aebdd140cebd540469c0de87960457a0ab81", commits[0].SHA)
	assert.Equal(t, "f6211fabb4e7f31f76b64dda1aa8545ff7eb2e78", commits[1].SHA)
	assert.Equal(t, "443bb80f40ada270ea0d8b9df4ce7f1173a5748a", commits[2].SHA)
}

func TestAPIMergedPullCommits(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 7}).(*models.PullRequest)
	assert.NoError(t, pullIssue.LoadIssue())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.HeadRepoID}).(*models.Repository)

	// test ListPullReviews
	session := loginUser(t, "user2")
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits", repo.OwnerName, repo.Name, pullIssue.Index)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var commits []*api.Commit
	DecodeJSON(t, resp, &commits)
	if !assert.Len(t, commits, 2) {
		return
	}
	assert.Equal(t, "e3dfaeb234e2a4570875ea3be7cedaebf350d703", commits[0].SHA)
	assert.Equal(t, "a20333be2715060c61c8d0268c4f622bde1ae4ac", commits[1].SHA)
}

func TestAPIClosedPullCommits(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 8}).(*models.PullRequest)
	assert.NoError(t, pullIssue.LoadIssue())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.HeadRepoID}).(*models.Repository)

	// test ListPullReviews
	session := loginUser(t, "user2")
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits", repo.OwnerName, repo.Name, pullIssue.Index)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var commits []*api.Commit
	DecodeJSON(t, resp, &commits)
	if !assert.Len(t, commits, 2) {
		return
	}
	assert.Equal(t, "f611a4c2dc1894ebbe5e8b558b4055169506048b", commits[0].SHA)
	assert.Equal(t, "e1adcf142a07305ebf40590a41b1a279486759db", commits[1].SHA)
}
