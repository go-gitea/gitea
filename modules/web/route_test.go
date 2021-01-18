// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
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
	assert.EqualValues(t, recorder.Code, http.StatusOK)
}

func TestRoute2(t *testing.T) {
	buff := bytes.NewBufferString("")
	recorder := httptest.NewRecorder()
	recorder.Body = buff

	var route int

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
				route = 0
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
				route = 1
			})
		}, func(resp http.ResponseWriter, req *http.Request) {
			resp.WriteHeader(200)
		})

		r.Group("/issues/{index}", func() {
			r.Get("/view", func(resp http.ResponseWriter, req *http.Request) {
				username := chi.URLParam(req, "username")
				assert.EqualValues(t, "gitea", username)
				reponame := chi.URLParam(req, "reponame")
				assert.EqualValues(t, "gitea", reponame)
				index := chi.URLParam(req, "index")
				assert.EqualValues(t, "1", index)
				route = 2
			})
		})
	})

	req, err := http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, recorder.Code, http.StatusOK)
	assert.EqualValues(t, 0, route)

	req, err = http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues/1", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, recorder.Code, http.StatusOK)
	assert.EqualValues(t, 1, route)

	req, err = http.NewRequest("GET", "http://localhost:8000/gitea/gitea/issues/1/view", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.EqualValues(t, recorder.Code, http.StatusOK)
	assert.EqualValues(t, 2, route)
}
