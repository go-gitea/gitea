// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

type APITestContext struct {
	Reponame     string
	Session      *TestSession
	Token        string
	Username     string
	ExpectedCode int
}

func NewAPITestContext(t *testing.T, username, reponame string) APITestContext {
	session := loginUser(t, username)
	token := getTokenForLoggedInUser(t, session)
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
			Gitignores:  "",
			License:     "WTFPL",
			Readme:      "Default",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos?token="+ctx.Token, createRepoOption)
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

func doAPIAddCollaborator(ctx APITestContext, username string, mode models.AccessMode) func(*testing.T) {
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
		req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s?token=%s", ctx.Username, ctx.Reponame, username, ctx.Token), addCollaboratorOption)
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
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks?token=%s", username, ctx.Reponame, ctx.Token), createForkOption)
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
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", ctx.Username, ctx.Reponame, ctx.Token)

		req := NewRequest(t, "GET", urlStr)
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
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", ctx.Username, ctx.Reponame, ctx.Token)

		req := NewRequest(t, "DELETE", urlStr)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPICreateUserKey(ctx APITestContext, keyname, keyFile string, callback ...func(*testing.T, api.PublicKey)) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/user/keys?token=%s", ctx.Token)

		dataPubKey, err := ioutil.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateKeyOption{
			Title: keyname,
			Key:   string(dataPubKey),
		})
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
		urlStr := fmt.Sprintf("/api/v1/user/keys/%d?token=%s", keyID, ctx.Token)

		req := NewRequest(t, "DELETE", urlStr)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func doAPICreateDeployKey(ctx APITestContext, keyname, keyFile string, readOnly bool) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", ctx.Username, ctx.Reponame, ctx.Token)

		dataPubKey, err := ioutil.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", urlStr, api.CreateKeyOption{
			Title:    keyname,
			Key:      string(dataPubKey),
			ReadOnly: readOnly,
		})

		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusCreated)
	}
}

func doAPICreatePullRequest(ctx APITestContext, owner, repo, baseBranch, headBranch string) func(*testing.T) (api.PullRequest, error) {
	return func(t *testing.T) (api.PullRequest, error) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls?token=%s",
			owner, repo, ctx.Token)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &api.CreatePullRequestOption{
			Head:  headBranch,
			Base:  baseBranch,
			Title: fmt.Sprintf("create a pr from %s to %s", headBranch, baseBranch),
		})

		expected := 201
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
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge?token=%s",
			owner, repo, index, ctx.Token)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &auth.MergePullRequestForm{
			MergeMessageField: "doAPIMergePullRequest Merge",
			Do:                string(models.MergeStyleMerge),
		})

		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, 200)
	}
}

func doAPIGetBranch(ctx APITestContext, branch string, callback ...func(*testing.T, api.Branch)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/branches/%s?token=%s", ctx.Username, ctx.Reponame, branch, ctx.Token)
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
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", ctx.Username, ctx.Reponame, treepath, ctx.Token)
		req := NewRequestWithJSON(t, "POST", url, &options)
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
