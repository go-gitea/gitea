// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRoute1(t *testing.T) {
	buff := bytes.NewBufferString("")
	recorder := httptest.NewRecorder()
	recorder.Body = buff

	r := NewRoute()
	r.Get("/{username}/{reponame}/{type:issues|pulls}", func(resp http.ResponseWriter, req *http.Request) {
		username := chi.URLParam(req, "username")
		assert.EqualValues(t, "gitea", username)
		reponame := chi.URLParam(req, "reponame")
		assert.EqualValues(t, "gitea", reponame)
		tp := chi.URLParam(req, "type")
		assert.EqualValues(t, "issues", tp)
	})

	req, err := http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
}

func TestRoute2(t *testing.T) {
	buff := bytes.NewBufferString("")
	recorder := httptest.NewRecorder()
	recorder.Body = buff

	hit := -1

	r := NewRoute()
	r.Group("/{username}/{reponame}", func() {
		r.Group("", func() {
			r.Get("/{type:issues|pulls}", func(resp http.ResponseWriter, req *http.Request) {
				username := chi.URLParam(req, "username")
				assert.EqualValues(t, "gitea", username)
				reponame := chi.URLParam(req, "reponame")
				assert.EqualValues(t, "gitea", reponame)
				tp := chi.URLParam(req, "type")
				assert.EqualValues(t, "issues", tp)
				hit = 0
			})

			r.Get("/{type:issues|pulls}/{index}", func(resp http.ResponseWriter, req *http.Request) {
				username := chi.URLParam(req, "username")
				assert.EqualValues(t, "gitea", username)
				reponame := chi.URLParam(req, "reponame")
				assert.EqualValues(t, "gitea", reponame)
				tp := chi.URLParam(req, "type")
				assert.EqualValues(t, "issues", tp)
				index := chi.URLParam(req, "index")
				assert.EqualValues(t, "1", index)
				hit = 1
			})
		}, func(resp http.ResponseWriter, req *http.Request) {
			if stop, err := strconv.Atoi(req.FormValue("stop")); err == nil {
				hit = stop
				resp.WriteHeader(http.StatusOK)
			}
		})

		r.Group("/issues/{index}", func() {
			r.Get("/view", func(resp http.ResponseWriter, req *http.Request) {
				username := chi.URLParam(req, "username")
				assert.EqualValues(t, "gitea", username)
				reponame := chi.URLParam(req, "reponame")
				assert.EqualValues(t, "gitea", reponame)
				index := chi.URLParam(req, "index")
				assert.EqualValues(t, "1", index)
				hit = 2
			})
		})
	})

	req, err := http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 0, hit)

	req, err = http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues/1", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 1, hit)

	req, err = http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues/1?stop=100", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 100, hit)

	req, err = http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues/1/view", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 2, hit)
}

func TestRoute3(t *testing.T) {
	buff := bytes.NewBufferString("")
	recorder := httptest.NewRecorder()
	recorder.Body = buff

	hit := -1

	m := NewRoute()
	r := NewRoute()
	r.Mount("/api/v1", m)

	m.Group("/repos", func() {
		m.Group("/{username}/{reponame}", func() {
			m.Group("/branch_protections", func() {
				m.Get("", func(resp http.ResponseWriter, req *http.Request) {
					hit = 0
				})
				m.Post("", func(resp http.ResponseWriter, req *http.Request) {
					hit = 1
				})
				m.Group("/{name}", func() {
					m.Get("", func(resp http.ResponseWriter, req *http.Request) {
						hit = 2
					})
					m.Patch("", func(resp http.ResponseWriter, req *http.Request) {
						hit = 3
					})
					m.Delete("", func(resp http.ResponseWriter, req *http.Request) {
						hit = 4
					})
				})
			})
		})
	})

	req, err := http.NewRequest("GET", "http://localhost:8000/api/v1/repos/gitea/gitea/branch_protections", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 0, hit)

	req, err = http.NewRequest("POST", "http://localhost:8000/api/v1/repos/gitea/gitea/branch_protections", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code, http.StatusOK)
	assert.EqualValues(t, 1, hit)

	req, err = http.NewRequest("GET", "http://localhost:8000/api/v1/repos/gitea/gitea/branch_protections/master", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 2, hit)

	req, err = http.NewRequest("PATCH", "http://localhost:8000/api/v1/repos/gitea/gitea/branch_protections/master", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 3, hit)

	req, err = http.NewRequest("DELETE", "http://localhost:8000/api/v1/repos/gitea/gitea/branch_protections/master", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, http.StatusOK, recorder.Code)
	assert.EqualValues(t, 4, hit)
}

func TestRouteNormalizePath(t *testing.T) {
	type paths struct {
		EscapedPath, RawPath, Path string
	}
	testPath := func(reqPath string, expectedPaths paths) {
		recorder := httptest.NewRecorder()
		recorder.Body = bytes.NewBuffer(nil)

		actualPaths := paths{EscapedPath: "(none)", RawPath: "(none)", Path: "(none)"}
		r := NewRoute()
		r.Get("/*", func(resp http.ResponseWriter, req *http.Request) {
			actualPaths.EscapedPath = req.URL.EscapedPath()
			actualPaths.RawPath = req.URL.RawPath
			actualPaths.Path = req.URL.Path
		})

		req, err := http.NewRequest("GET", reqPath, nil)
		assert.NoError(t, err)
		r.ServeHTTP(recorder, req)
		assert.Equal(t, expectedPaths, actualPaths, "req path = %q", reqPath)
	}

	// RawPath could be empty if the EscapedPath is the same as escape(Path) and it is already normalized
	testPath("/", paths{EscapedPath: "/", RawPath: "", Path: "/"})
	testPath("//", paths{EscapedPath: "/", RawPath: "/", Path: "/"})
	testPath("/%2f", paths{EscapedPath: "/%2f", RawPath: "/%2f", Path: "//"})
	testPath("///a//b/", paths{EscapedPath: "/a/b", RawPath: "/a/b", Path: "/a/b"})

	defer test.MockVariableValue(&setting.UseSubURLPath, true)()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub-path")()
	testPath("/", paths{EscapedPath: "(none)", RawPath: "(none)", Path: "(none)"}) // 404
	testPath("/sub-path", paths{EscapedPath: "/", RawPath: "/", Path: "/"})
	testPath("/sub-path/", paths{EscapedPath: "/", RawPath: "/", Path: "/"})
	testPath("/sub-path//a/b///", paths{EscapedPath: "/a/b", RawPath: "/a/b", Path: "/a/b"})
	testPath("/sub-path/%2f/", paths{EscapedPath: "/%2f", RawPath: "/%2f", Path: "//"})
	// "/v2" is special for OCI container registry, it should always be in the root of the site
	testPath("/v2", paths{EscapedPath: "/v2", RawPath: "/v2", Path: "/v2"})
	testPath("/v2/", paths{EscapedPath: "/v2", RawPath: "/v2", Path: "/v2"})
	testPath("/v2/%2f", paths{EscapedPath: "/v2/%2f", RawPath: "/v2/%2f", Path: "/v2//"})
}
