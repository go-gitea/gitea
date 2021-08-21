// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"
	"github.com/stretchr/testify/assert"
)

type TestContext struct {
	Reponame     string
	Session      *TestSession
	Username     string
	ExpectedCode int
}

func NewTestContext(t *testing.T, username, reponame string) TestContext {
	return TestContext{
		Session:  loginUser(t, username),
		Username: username,
		Reponame: reponame,
	}
}

func (ctx TestContext) GitPath() string {
	return fmt.Sprintf("%s/%s.git", ctx.Username, ctx.Reponame)
}

func (ctx TestContext) CreateAPITestContext(t *testing.T) APITestContext {
	return NewAPITestContext(t, ctx.Username, ctx.Reponame)
}

func doCreateRepository(ctx TestContext, empty bool, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		createRepoOption := &api.CreateRepoOption{
			AutoInit:    !empty,
			Description: "Temporary repo",
			Name:        ctx.Reponame,
			Private:     true,
			Template:    true,
			Gitignores:  "",
			License:     "WTFPL",
			Readme:      "Default",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", createRepoOption)
		apiCtx := ctx.CreateAPITestContext(t)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := apiCtx.MakeRequest(t, req, http.StatusCreated)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doDeleteRepository(ctx TestContext) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s", ctx.Username, ctx.Reponame)
		apiCtx := ctx.CreateAPITestContext(t)
		req := NewRequest(t, "DELETE", urlStr)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		apiCtx.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAddCollaborator(ctx TestContext, username string, mode models.AccessMode) func(*testing.T) {
	return func(t *testing.T) {
		permission := "read"

		if mode == models.AccessModeAdmin {
			permission = "admin"
		} else if mode > models.AccessModeRead {
			permission = "write"
		}
		addCollaboratorOption := &api.AddCollaboratorOption{
			Permission: &permission,
		}
		apiCtx := ctx.CreateAPITestContext(t)
		req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", ctx.Username, ctx.Reponame, username), addCollaboratorOption)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		apiCtx.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doForkRepository(ctx TestContext, username string, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		createForkOption := &api.CreateForkOption{}
		apiCtx := ctx.CreateAPITestContext(t)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", username, ctx.Reponame), createForkOption)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := apiCtx.MakeRequest(t, req, http.StatusAccepted)
		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doEditRepository(ctx TestContext, editRepoOption *api.EditRepoOption, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		apiCtx := ctx.CreateAPITestContext(t)
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), editRepoOption)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := apiCtx.MakeRequest(t, req, http.StatusOK)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doCreatePullRequest(ctx TestContext, owner, repo, baseBranch, headBranch string) func(*testing.T) (api.PullRequest, error) {
	return func(t *testing.T) (api.PullRequest, error) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner, repo)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &api.CreatePullRequestOption{
			Head:  headBranch,
			Base:  baseBranch,
			Title: fmt.Sprintf("create a pr from %s to %s", headBranch, baseBranch),
		})

		expected := 201
		if ctx.ExpectedCode != 0 {
			expected = ctx.ExpectedCode
		}
		apiCtx := ctx.CreateAPITestContext(t)
		resp := apiCtx.MakeRequest(t, req, expected)

		decoder := json.NewDecoder(resp.Body)
		pr := api.PullRequest{}
		err := decoder.Decode(&pr)
		return pr, err
	}
}

func doCreateUserKey(ctx TestContext, keyname, keyFile string, callback ...func(*testing.T, api.PublicKey)) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := "/api/v1/user/keys"

		dataPubKey, err := ioutil.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateKeyOption{
			Title: keyname,
			Key:   string(dataPubKey),
		})
		apiCtx := ctx.CreateAPITestContext(t)
		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := apiCtx.MakeRequest(t, req, http.StatusCreated)
		var publicKey api.PublicKey
		DecodeJSON(t, resp, &publicKey)
		if len(callback) > 0 {
			callback[0](t, publicKey)
		}
	}
}

func doGetPullRequest(ctx TestContext, owner, repo string, index int64) func(*testing.T) (api.PullRequest, error) {
	return func(t *testing.T) (api.PullRequest, error) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner, repo, index)
		req := NewRequest(t, http.MethodGet, urlStr)

		expected := 200
		if ctx.ExpectedCode != 0 {
			expected = ctx.ExpectedCode
		}
		apiCtx := ctx.CreateAPITestContext(t)
		resp := apiCtx.MakeRequest(t, req, expected)

		decoder := json.NewDecoder(resp.Body)
		pr := api.PullRequest{}
		err := decoder.Decode(&pr)
		return pr, err
	}
}

func doMergePullRequest(ctx TestContext, owner, repo string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge",
			owner, repo, index)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
			MergeMessageField: "doAPIMergePullRequest Merge",
			Do:                string(models.MergeStyleMerge),
		})

		apiCtx := ctx.CreateAPITestContext(t)
		resp := apiCtx.MakeRequest(t, req, NoExpectedStatus)

		if resp.Code == http.StatusMethodNotAllowed {
			err := api.APIError{}
			DecodeJSON(t, resp, &err)
			assert.EqualValues(t, "Please try again later", err.Message)
			queue.GetManager().FlushAll(context.Background(), 5*time.Second)
			req = NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
				MergeMessageField: "doAPIMergePullRequest Merge",
				Do:                string(models.MergeStyleMerge),
			})
			resp = apiCtx.MakeRequest(t, req, NoExpectedStatus)
		}

		expected := ctx.ExpectedCode
		if expected == 0 {
			expected = 200
		}

		if !assert.EqualValues(t, expected, resp.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, resp)
		}
	}
}

func doManuallyMergePullRequest(ctx TestContext, owner, repo, commitID string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge",
			owner, repo, index)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
			Do:            string(models.MergeStyleManuallyMerged),
			MergeCommitID: commitID,
		})

		apiCtx := ctx.CreateAPITestContext(t)

		if ctx.ExpectedCode != 0 {
			apiCtx.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		apiCtx.MakeRequest(t, req, 200)
	}
}
