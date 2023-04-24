// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	scontext "context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web/middleware"

	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// MockContext mock context for unit tests
func MockContext(t *testing.T, path string) *context.Context {
	resp := &mockResponseWriter{}
	ctx := context.Context{
		Render: &mockRender{},
		Data:   make(map[string]interface{}),
		Flash: &middleware.Flash{
			Values: make(url.Values),
		},
		Resp:   context.NewResponse(resp),
		Locale: &translation.MockLocale{},
	}
	defer ctx.Close()

	requestURL, err := url.Parse(path)
	assert.NoError(t, err)
	req := &http.Request{
		URL:  requestURL,
		Form: url.Values{},
	}

	chiCtx := chi.NewRouteContext()
	req = req.WithContext(scontext.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	ctx.Req = context.WithContext(req, &ctx)
	return &ctx
}

// LoadRepo load a repo into a test context.
func LoadRepo(t *testing.T, ctx *context.Context, repoID int64) {
	ctx.Repo = &context.Repository{}
	ctx.Repo.Repository = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	var err error
	ctx.Repo.Owner, err = user_model.GetUserByID(ctx, ctx.Repo.Repository.OwnerID)
	assert.NoError(t, err)
	ctx.Repo.RepoLink = ctx.Repo.Repository.Link()
	ctx.Repo.Permission, err = access_model.GetUserRepoPermission(ctx, ctx.Repo.Repository, ctx.Doer)
	assert.NoError(t, err)
}

// LoadRepoCommit loads a repo's commit into a test context.
func LoadRepoCommit(t *testing.T, ctx *context.Context) {
	gitRepo, err := git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	branch, err := gitRepo.GetHEADBranch()
	assert.NoError(t, err)
	assert.NotNil(t, branch)
	if branch != nil {
		ctx.Repo.Commit, err = gitRepo.GetBranchCommit(branch.Name)
		assert.NoError(t, err)
	}
}

// LoadUser load a user into a test context.
func LoadUser(t *testing.T, ctx *context.Context, userID int64) {
	ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
}

// LoadGitRepo load a git repo into a test context. Requires that ctx.Repo has
// already been populated.
func LoadGitRepo(t *testing.T, ctx *context.Context) {
	assert.NoError(t, ctx.Repo.Repository.LoadOwner(ctx))
	var err error
	ctx.Repo.GitRepo, err = git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
}

type mockResponseWriter struct {
	httptest.ResponseRecorder
	size int
}

func (rw *mockResponseWriter) Write(b []byte) (int, error) {
	rw.size += len(b)
	return rw.ResponseRecorder.Write(b)
}

func (rw *mockResponseWriter) Status() int {
	return rw.ResponseRecorder.Code
}

func (rw *mockResponseWriter) Written() bool {
	return rw.ResponseRecorder.Code > 0
}

func (rw *mockResponseWriter) Size() int {
	return rw.size
}

func (rw *mockResponseWriter) Push(target string, opts *http.PushOptions) error {
	return nil
}

type mockRender struct{}

func (tr *mockRender) TemplateLookup(tmpl string) (templates.TemplateExecutor, error) {
	return nil, nil
}

func (tr *mockRender) HTML(w io.Writer, status int, _ string, _ interface{}) error {
	if resp, ok := w.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
	return nil
}
