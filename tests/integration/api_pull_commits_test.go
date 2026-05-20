// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIPullCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.HeadRepoID})

	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits", repo.OwnerName, repo.Name, pr.Index)
	resp := MakeRequest(t, req, http.StatusOK)

	commits := DecodeJSON(t, resp, []*api.Commit{})

	require.Len(t, commits, 2)

	assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", commits[0].SHA)
	assert.Equal(t, "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2", commits[1].SHA)

	assert.NotEmpty(t, commits[0].Files)
	assert.NotEmpty(t, commits[1].Files)
	assert.NotNil(t, commits[0].RepoCommit.Verification)
	assert.NotNil(t, commits[1].RepoCommit.Verification)
}

// TestAPIPullCommitsNotDuplicatedViaMergePaths is a regression test for:
// https://github.com/go-gitea/gitea/issues/37383
//
// When the same commit reaches a branch via two different merge paths, Gitea
// must not list it again as a new commit in a subsequent PR targeting that branch.
//
// Git topology:
//
//	main:    A
//	feature: A → B          (B = feature file commit)
//	staging: A → M1         (M1 = merge commit "feature into staging"; parents: A, B)
//	develop: A → M2         (M2 = merge commit "feature into develop";  parents: A, B)
//
// PR develop → staging (PR3):
//
//	git log staging..develop = [M2]     correct: M2 is new; B is already in staging via M1
//	git log staging..develop = [M2, B]  buggy: B incorrectly listed as new
func TestAPIPullCommitsNotDuplicatedViaMergePaths(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := NewAPITestContext(t, "user2", "pr-commits-dupe-test")
		doAPICreateRepository(ctx, false)(t)

		// Create branches from the default branch
		for _, branch := range []string{"staging", "develop", "feature"} {
			req := NewRequestWithJSON(t, http.MethodPost,
				"/api/v1/repos/"+ctx.Username+"/"+ctx.Reponame+"/branches",
				&api.CreateBranchRepoOption{BranchName: branch},
			).AddTokenAuth(ctx.Token)
			ctx.Session.MakeRequest(t, req, http.StatusCreated)
		}

		// Commit a file on the feature branch (commit B in the diagram above)
		doAPICreateFile(ctx, "feature.txt", &api.CreateFileOptions{
			FileOptions:   api.FileOptions{Message: "add feature.txt", BranchName: "feature"},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("feature content")),
		})(t)

		// PR1: feature → staging — merge creates M1
		pr1, err := doAPICreatePullRequest(ctx, ctx.Username, ctx.Reponame, "staging", "feature")(t)
		require.NoError(t, err)
		doAPIMergePullRequest(ctx, ctx.Username, ctx.Reponame, pr1.Index)(t)

		// PR2: feature → develop — merge creates M2
		pr2, err := doAPICreatePullRequest(ctx, ctx.Username, ctx.Reponame, "develop", "feature")(t)
		require.NoError(t, err)
		doAPIMergePullRequest(ctx, ctx.Username, ctx.Reponame, pr2.Index)(t)

		// Flush queues so that PR3's merge-base computation starts from a clean state
		queue.GetManager().FlushAll(t.Context(), 5*time.Second)

		// PR3: develop → staging — the PR whose commit list is under test
		pr3, err := doAPICreatePullRequest(ctx, ctx.Username, ctx.Reponame, "staging", "develop")(t)
		require.NoError(t, err)

		req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits",
			ctx.Username, ctx.Reponame, pr3.Index,
		).AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var commits []*api.Commit
		DecodeJSON(t, resp, &commits)

		// M2 is genuinely new in develop and must appear.
		// B must NOT appear — it is already reachable from staging via M1.
		// Buggy Gitea returns [M2, B] (len 2); fixed Gitea returns [M2] (len 1).
		require.Len(t, commits, 1, "PR develop→staging must not list commits already present in staging")
	})
}

// TODO add tests for already merged PR and closed PR
