// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

type APITestContext struct {
	Reponame     string
	Session      *TestSession
	Token        string
	Username     string
	ExpectedCode int
}

func NewAPITestContext(t *testing.T, username, reponame string, scope ...auth.AccessTokenScope) APITestContext {
	session := loginUser(t, username)
	if len(scope) == 0 {
		// FIXME: legacy logic: no scope means all
		scope = []auth.AccessTokenScope{auth.AccessTokenScopeAll}
	}
	token := getTokenForLoggedInUser(t, session, scope...)
	return APITestContext{
		Session:  session,
		Token:    token,
		Username: username,
		Reponame: reponame,
	}
}

func (ctx APITestContext) GitPath() string {
	return fmt.Sprintf("%s/%s.git", ctx.Username, ctx.Reponame)
}

func doAPICreateRepository(ctx APITestContext, empty bool, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
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
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", createRepoOption).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doAPIEditRepository(ctx APITestContext, editRepoOption *api.EditRepoOption, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), editRepoOption).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doAPIAddCollaborator(ctx APITestContext, username string, mode perm.AccessMode) func(*testing.T) {
	return func(t *testing.T) {
		permission := "read"

		if mode == perm.AccessModeAdmin {
			permission = "admin"
		} else if mode > perm.AccessModeRead {
			permission = "write"
		}
		addCollaboratorOption := &api.AddCollaboratorOption{
			Permission: &permission,
		}
		req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", ctx.Username, ctx.Reponame, username), addCollaboratorOption).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPIForkRepository(ctx APITestContext, username string, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		createForkOption := &api.CreateForkOption{}
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", username, ctx.Reponame), createForkOption).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusAccepted)
		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doAPIGetRepository(ctx APITestContext, callback ...func(*testing.T, api.Repository)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", ctx.Username, ctx.Reponame)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)
		if len(callback) > 0 {
			callback[0](t, repository)
		}
	}
}

func doAPIDeleteRepository(ctx APITestContext) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s", ctx.Username, ctx.Reponame)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPICreateUserKey(ctx APITestContext, keyname, keyFile string, callback ...func(*testing.T, api.PublicKey)) func(*testing.T) {
	return func(t *testing.T) {
		dataPubKey, err := os.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/keys", &api.CreateKeyOption{
			Title: keyname,
			Key:   string(dataPubKey),
		}).AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)
		var publicKey api.PublicKey
		DecodeJSON(t, resp, &publicKey)
		if len(callback) > 0 {
			callback[0](t, publicKey)
		}
	}
}

func doAPIDeleteUserKey(ctx APITestContext, keyID int64) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/user/keys/%d", keyID)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPICreateDeployKey(ctx APITestContext, keyname, keyFile string, readOnly bool) func(*testing.T) {
	return func(t *testing.T) {
		dataPubKey, err := os.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/keys", ctx.Username, ctx.Reponame), api.CreateKeyOption{
			Title:    keyname,
			Key:      string(dataPubKey),
			ReadOnly: readOnly,
		}).AddTokenAuth(ctx.Token)

		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusCreated)
	}
}

func doAPICreatePullRequest(ctx APITestContext, owner, repo, baseBranch, headBranch string) func(*testing.T) (api.PullRequest, error) {
	return func(t *testing.T) (api.PullRequest, error) {
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner, repo), &api.CreatePullRequestOption{
			Head:  headBranch,
			Base:  baseBranch,
			Title: fmt.Sprintf("create a pr from %s to %s", headBranch, baseBranch),
		}).AddTokenAuth(ctx.Token)

		expected := http.StatusCreated
		if ctx.ExpectedCode != 0 {
			expected = ctx.ExpectedCode
		}
		resp := ctx.Session.MakeRequest(t, req, expected)

		decoder := json.NewDecoder(resp.Body)
		pr := api.PullRequest{}
		err := decoder.Decode(&pr)
		return pr, err
	}
}

func doAPIGetPullRequest(ctx APITestContext, owner, repo string, index int64) func(*testing.T) (api.PullRequest, error) {
	return func(t *testing.T) (api.PullRequest, error) {
		req := NewRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner, repo, index)).
			AddTokenAuth(ctx.Token)

		expected := http.StatusOK
		if ctx.ExpectedCode != 0 {
			expected = ctx.ExpectedCode
		}
		resp := ctx.Session.MakeRequest(t, req, expected)

		decoder := json.NewDecoder(resp.Body)
		pr := api.PullRequest{}
		err := decoder.Decode(&pr)
		return pr, err
	}
}

func doAPIMergePullRequest(ctx APITestContext, owner, repo string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner, repo, index)

		var req *RequestWrapper
		var resp *httptest.ResponseRecorder

		for i := 0; i < 6; i++ {
			req = NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
				MergeMessageField: "doAPIMergePullRequest Merge",
				Do:                string(repo_model.MergeStyleMerge),
			}).AddTokenAuth(ctx.Token)

			resp = ctx.Session.MakeRequest(t, req, NoExpectedStatus)

			if resp.Code != http.StatusMethodNotAllowed {
				break
			}
			err := api.APIError{}
			DecodeJSON(t, resp, &err)
			assert.EqualValues(t, "Please try again later", err.Message)
			queue.GetManager().FlushAll(t.Context(), 5*time.Second)
			<-time.After(1 * time.Second)
		}

		expected := ctx.ExpectedCode
		if expected == 0 {
			expected = http.StatusOK
		}

		if !assert.EqualValues(t, expected, resp.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, resp)
		}
	}
}

func doAPIManuallyMergePullRequest(ctx APITestContext, owner, repo, commitID string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner, repo, index)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
			Do:            string(repo_model.MergeStyleManuallyMerged),
			MergeCommitID: commitID,
		}).AddTokenAuth(ctx.Token)

		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}

func doAPIAutoMergePullRequest(ctx APITestContext, owner, repo string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner, repo, index)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
			MergeMessageField:      "doAPIMergePullRequest Merge",
			Do:                     string(repo_model.MergeStyleMerge),
			MergeWhenChecksSucceed: true,
		}).AddTokenAuth(ctx.Token)

		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}

func doAPICancelAutoMergePullRequest(ctx APITestContext, owner, repo string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner, repo, index)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPIGetBranch(ctx APITestContext, branch string, callback ...func(*testing.T, api.Branch)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/branches/%s", ctx.Username, ctx.Reponame, branch).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var branch api.Branch
		DecodeJSON(t, resp, &branch)
		if len(callback) > 0 {
			callback[0](t, branch)
		}
	}
}

func doAPICreateFile(ctx APITestContext, treepath string, options *api.CreateFileOptions, callback ...func(*testing.T, api.FileResponse)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", ctx.Username, ctx.Reponame, treepath), &options).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)

		var contents api.FileResponse
		DecodeJSON(t, resp, &contents)
		if len(callback) > 0 {
			callback[0](t, contents)
		}
	}
}

func doAPICreateOrganization(ctx APITestContext, options *api.CreateOrgOption, callback ...func(*testing.T, api.Organization)) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &options).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)

		var contents api.Organization
		DecodeJSON(t, resp, &contents)
		if len(callback) > 0 {
			callback[0](t, contents)
		}
	}
}

func doAPICreateOrganizationRepository(ctx APITestContext, orgName string, options *api.CreateRepoOption, callback ...func(*testing.T, api.Repository)) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &options).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)

		var contents api.Repository
		DecodeJSON(t, resp, &contents)
		if len(callback) > 0 {
			callback[0](t, contents)
		}
	}
}

func doAPICreateOrganizationTeam(ctx APITestContext, orgName string, options *api.CreateTeamOption, callback ...func(*testing.T, api.Team)) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgName), &options).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		resp := ctx.Session.MakeRequest(t, req, http.StatusCreated)

		var contents api.Team
		DecodeJSON(t, resp, &contents)
		if len(callback) > 0 {
			callback[0](t, contents)
		}
	}
}

func doAPIAddUserToOrganizationTeam(ctx APITestContext, teamID int64, username string) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/members/%s", teamID, username)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPIAddRepoToOrganizationTeam(ctx APITestContext, teamID int64, orgName, repoName string) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", teamID, orgName, repoName)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}
