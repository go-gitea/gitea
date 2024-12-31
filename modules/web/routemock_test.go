// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRouteMock(t *testing.T) {
	setting.IsInTesting = true

	r := NewRouter()
	middleware1 := func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("X-Test-Middleware1", "m1")
	}
	middleware2 := func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("X-Test-Middleware2", "m2")
	}
	handler := func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("X-Test-Handler", "h")
	}
	r.Get("/foo", middleware1, RouterMockPoint("mock-point"), middleware2, handler)

	// normal request
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://localhost:8000/foo", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.Len(t, recorder.Header(), 3)
	assert.EqualValues(t, "m1", recorder.Header().Get("X-Test-Middleware1"))
	assert.EqualValues(t, "m2", recorder.Header().Get("X-Test-Middleware2"))
	assert.EqualValues(t, "h", recorder.Header().Get("X-Test-Handler"))
	RouteMockReset()

	// mock at "mock-point"
	RouteMock("mock-point", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("X-Test-MockPoint", "a")
		resp.WriteHeader(http.StatusOK)
	})
	recorder = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://localhost:8000/foo", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.Len(t, recorder.Header(), 2)
	assert.EqualValues(t, "m1", recorder.Header().Get("X-Test-Middleware1"))
	assert.EqualValues(t, "a", recorder.Header().Get("X-Test-MockPoint"))
	RouteMockReset()

	// mock at MockAfterMiddlewares
	RouteMock(MockAfterMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("X-Test-MockPoint", "b")
		resp.WriteHeader(http.StatusOK)
	})
	recorder = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://localhost:8000/foo", nil)
	assert.NoError(t, err)
	r.ServeHTTP(recorder, req)
	assert.Len(t, recorder.Header(), 3)
	assert.EqualValues(t, "m1", recorder.Header().Get("X-Test-Middleware1"))
	assert.EqualValues(t, "m2", recorder.Header().Get("X-Test-Middleware2"))
	assert.EqualValues(t, "b", recorder.Header().Get("X-Test-MockPoint"))
	RouteMockReset()
}
