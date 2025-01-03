// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func chiURLParamsToMap(chiCtx *chi.Context) map[string]string {
	pathParams := chiCtx.URLParams
	m := make(map[string]string, len(pathParams.Keys))
	for i, key := range pathParams.Keys {
		if key == "*" && pathParams.Values[i] == "" {
			continue // chi router will add an empty "*" key if there is a "Mount"
		}
		m[key] = pathParams.Values[i]
	}
	return util.Iif(len(m) == 0, nil, m)
}

func TestPathProcessor(t *testing.T) {
	testProcess := func(pattern, uri string, expectedPathParams map[string]string) {
		chiCtx := chi.NewRouteContext()
		chiCtx.RouteMethod = "GET"
		p := newRouterPathMatcher("GET", pattern, http.NotFound)
		assert.True(t, p.matchPath(chiCtx, uri), "use pattern %s to process uri %s", pattern, uri)
		assert.Equal(t, expectedPathParams, chiURLParamsToMap(chiCtx), "use pattern %s to process uri %s", pattern, uri)
	}

	// the "<...>" is intentionally designed to distinguish from chi's path parameters, because:
	// 1. their behaviors are totally different, we do not want to mislead developers
	// 2. we can write regexp in "<name:\w{3,4}>" easily and parse it easily
	testProcess("/<p1>/<p2>", "/a/b", map[string]string{"p1": "a", "p2": "b"})
	testProcess("/<p1:*>", "", map[string]string{"p1": ""}) // this is a special case, because chi router could use empty path
	testProcess("/<p1:*>", "/", map[string]string{"p1": ""})
	testProcess("/<p1:*>/<p2>", "/a", map[string]string{"p1": "", "p2": "a"})
	testProcess("/<p1:*>/<p2>", "/a/b", map[string]string{"p1": "a", "p2": "b"})
	testProcess("/<p1:*>/<p2>", "/a/b/c", map[string]string{"p1": "a/b", "p2": "c"})
}

func TestRouter(t *testing.T) {
	buff := bytes.NewBufferString("")
	recorder := httptest.NewRecorder()
	recorder.Body = buff

	type resultStruct struct {
		method      string
		pathParams  map[string]string
		handlerMark string
	}
	var res resultStruct

	h := func(optMark ...string) func(resp http.ResponseWriter, req *http.Request) {
		mark := util.OptionalArg(optMark, "")
		return func(resp http.ResponseWriter, req *http.Request) {
			res.method = req.Method
			res.pathParams = chiURLParamsToMap(chi.RouteContext(req.Context()))
			res.handlerMark = mark
		}
	}

	stopMark := func(optMark ...string) func(resp http.ResponseWriter, req *http.Request) {
		mark := util.OptionalArg(optMark, "")
		return func(resp http.ResponseWriter, req *http.Request) {
			if stop := req.FormValue("stop"); stop != "" && (mark == "" || mark == stop) {
				h(stop)(resp, req)
				resp.WriteHeader(http.StatusOK)
			}
		}
	}

	r := NewRouter()
	r.NotFound(h("not-found:/"))
	r.Get("/{username}/{reponame}/{type:issues|pulls}", h("list-issues-a")) // this one will never be called
	r.Group("/{username}/{reponame}", func() {
		r.Get("/{type:issues|pulls}", h("list-issues-b"))
		r.Group("", func() {
			r.Get("/{type:issues|pulls}/{index}", h("view-issue"))
		}, stopMark())
		r.Group("/issues/{index}", func() {
			r.Post("/update", h("update-issue"))
		})
	})

	m := NewRouter()
	m.NotFound(h("not-found:/api/v1"))
	r.Mount("/api/v1", m)
	m.Group("/repos", func() {
		m.Group("/{username}/{reponame}", func() {
			m.Group("/branches", func() {
				m.Get("", h())
				m.Post("", h())
				m.Group("/{name}", func() {
					m.Get("", h())
					m.Patch("", h())
					m.Delete("", h())
				})
				m.PathGroup("/*", func(g *RouterPathGroup) {
					g.MatchPath("GET", `/<dir:*>/<file:[a-z]{1,2}>`, stopMark("s2"), h("match-path"))
				}, stopMark("s1"))
			})
		})
	})

	testRoute := func(t *testing.T, methodPath string, expected resultStruct) {
		t.Run(methodPath, func(t *testing.T) {
			res = resultStruct{}
			methodPathFields := strings.Fields(methodPath)
			req, err := http.NewRequest(methodPathFields[0], methodPathFields[1], nil)
			assert.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.EqualValues(t, expected, res)
		})
	}

	t.Run("RootRouter", func(t *testing.T) {
		testRoute(t, "GET /the-user/the-repo/other", resultStruct{method: "GET", handlerMark: "not-found:/"})
		testRoute(t, "GET /the-user/the-repo/pulls", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "type": "pulls"},
			handlerMark: "list-issues-b",
		})
		testRoute(t, "GET /the-user/the-repo/issues/123", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "type": "issues", "index": "123"},
			handlerMark: "view-issue",
		})
		testRoute(t, "GET /the-user/the-repo/issues/123?stop=hijack", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "type": "issues", "index": "123"},
			handlerMark: "hijack",
		})
		testRoute(t, "POST /the-user/the-repo/issues/123/update", resultStruct{
			method:      "POST",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "index": "123"},
			handlerMark: "update-issue",
		})
	})

	t.Run("Sub Router", func(t *testing.T) {
		testRoute(t, "GET /api/v1/other", resultStruct{method: "GET", handlerMark: "not-found:/api/v1"})
		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches", resultStruct{
			method:     "GET",
			pathParams: map[string]string{"username": "the-user", "reponame": "the-repo"},
		})

		testRoute(t, "POST /api/v1/repos/the-user/the-repo/branches", resultStruct{
			method:     "POST",
			pathParams: map[string]string{"username": "the-user", "reponame": "the-repo"},
		})

		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/master", resultStruct{
			method:     "GET",
			pathParams: map[string]string{"username": "the-user", "reponame": "the-repo", "name": "master"},
		})

		testRoute(t, "PATCH /api/v1/repos/the-user/the-repo/branches/master", resultStruct{
			method:     "PATCH",
			pathParams: map[string]string{"username": "the-user", "reponame": "the-repo", "name": "master"},
		})

		testRoute(t, "DELETE /api/v1/repos/the-user/the-repo/branches/master", resultStruct{
			method:     "DELETE",
			pathParams: map[string]string{"username": "the-user", "reponame": "the-repo", "name": "master"},
		})
	})

	t.Run("MatchPath", func(t *testing.T) {
		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/d1/d2/fn", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "*": "d1/d2/fn", "dir": "d1/d2", "file": "fn"},
			handlerMark: "match-path",
		})
		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/d1%2fd2/fn", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "*": "d1%2fd2/fn", "dir": "d1%2fd2", "file": "fn"},
			handlerMark: "match-path",
		})
		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/d1/d2/000", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"reponame": "the-repo", "username": "the-user", "*": "d1/d2/000"},
			handlerMark: "not-found:/api/v1",
		})

		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/d1/d2/fn?stop=s1", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "*": "d1/d2/fn"},
			handlerMark: "s1",
		})

		testRoute(t, "GET /api/v1/repos/the-user/the-repo/branches/d1/d2/fn?stop=s2", resultStruct{
			method:      "GET",
			pathParams:  map[string]string{"username": "the-user", "reponame": "the-repo", "*": "d1/d2/fn", "dir": "d1/d2", "file": "fn"},
			handlerMark: "s2",
		})
	})
}

func TestRouteNormalizePath(t *testing.T) {
	type paths struct {
		EscapedPath, RawPath, Path string
	}
	testPath := func(reqPath string, expectedPaths paths) {
		recorder := httptest.NewRecorder()
		recorder.Body = bytes.NewBuffer(nil)

		actualPaths := paths{EscapedPath: "(none)", RawPath: "(none)", Path: "(none)"}
		r := NewRouter()
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
