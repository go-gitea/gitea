// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"
	"github.com/stretchr/testify/assert"
)

// MockContext mock context for unit tests
func MockContext(t *testing.T, path string) *context.Context {
	var macaronContext macaron.Context
	macaronContext.ReplaceAllParams(macaron.Params{})
	macaronContext.Locale = &mockLocale{}
	requestURL, err := url.Parse(path)
	assert.NoError(t, err)
	macaronContext.Req = macaron.Request{Request: &http.Request{
		URL:  requestURL,
		Form: url.Values{},
	}}
	macaronContext.Resp = &mockResponseWriter{}
	macaronContext.Render = &mockRender{ResponseWriter: macaronContext.Resp}
	macaronContext.Data = map[string]interface{}{}
	return &context.Context{
		Context: &macaronContext,
		Flash: &session.Flash{
			Values: make(url.Values),
		},
	}
}

// LoadRepo load a repo into a test context.
func LoadRepo(t *testing.T, ctx *context.Context, repoID int64) {
	ctx.Repo = &context.Repository{}
	ctx.Repo.Repository = models.AssertExistsAndLoadBean(t, &models.Repository{ID: repoID}).(*models.Repository)
	ctx.Repo.RepoLink = ctx.Repo.Repository.Link()
	var err error
	ctx.Repo.Permission, err = models.GetUserRepoPermission(ctx.Repo.Repository, ctx.User)
	assert.NoError(t, err)
}

// LoadRepoCommit loads a repo's commit into a test context.
func LoadRepoCommit(t *testing.T, ctx *context.Context) {
	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	branch, err := gitRepo.GetHEADBranch()
	assert.NoError(t, err)
	ctx.Repo.Commit, err = gitRepo.GetBranchCommit(branch.Name)
	assert.NoError(t, err)
}

// LoadUser load a user into a test context.
func LoadUser(t *testing.T, ctx *context.Context, userID int64) {
	ctx.User = models.AssertExistsAndLoadBean(t, &models.User{ID: userID}).(*models.User)
}

// LoadGitRepo load a git repo into a test context. Requires that ctx.Repo has
// already been populated.
func LoadGitRepo(t *testing.T, ctx *context.Context) {
	assert.NoError(t, ctx.Repo.Repository.GetOwner())
	var err error
	ctx.Repo.GitRepo, err = git.OpenRepository(ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
}

type mockLocale struct{}

func (l mockLocale) Language() string {
	return "en"
}

func (l mockLocale) Tr(s string, _ ...interface{}) string {
	return s
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

func (rw *mockResponseWriter) Before(b macaron.BeforeFunc) {
	b(rw)
}

type mockRender struct {
	http.ResponseWriter
}

func (tr *mockRender) SetResponseWriter(rw http.ResponseWriter) {
	tr.ResponseWriter = rw
}

func (tr *mockRender) JSON(status int, _ interface{}) {
	tr.Status(status)
}

func (tr *mockRender) JSONString(interface{}) (string, error) {
	return "", nil
}

func (tr *mockRender) RawData(status int, _ []byte) {
	tr.Status(status)
}

func (tr *mockRender) PlainText(status int, _ []byte) {
	tr.Status(status)
}

func (tr *mockRender) HTML(status int, _ string, _ interface{}, _ ...macaron.HTMLOptions) {
	tr.Status(status)
}

func (tr *mockRender) HTMLSet(status int, _ string, _ string, _ interface{}, _ ...macaron.HTMLOptions) {
	tr.Status(status)
}

func (tr *mockRender) HTMLSetString(string, string, interface{}, ...macaron.HTMLOptions) (string, error) {
	return "", nil
}

func (tr *mockRender) HTMLString(string, interface{}, ...macaron.HTMLOptions) (string, error) {
	return "", nil
}

func (tr *mockRender) HTMLSetBytes(string, string, interface{}, ...macaron.HTMLOptions) ([]byte, error) {
	return nil, nil
}

func (tr *mockRender) HTMLBytes(string, interface{}, ...macaron.HTMLOptions) ([]byte, error) {
	return nil, nil
}

func (tr *mockRender) XML(status int, _ interface{}) {
	tr.Status(status)
}

func (tr *mockRender) Error(status int, _ ...string) {
	tr.Status(status)
}

func (tr *mockRender) Status(status int) {
	tr.ResponseWriter.WriteHeader(status)
}

func (tr *mockRender) SetTemplatePath(string, string) {
}

func (tr *mockRender) HasTemplateSet(string) bool {
	return true
}
