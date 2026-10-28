// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/modules/setting"

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

	for _, tc := range []struct {
		mockPoint, mockValue string
		headers              [][2]string
	}{
		// normal request
		{headers: [][2]string{{"X-Test-Middleware1", "m1"}, {"X-Test-Middleware2", "m2"}, {"X-Test-Handler", "h"}}},
		// mock at "mock-point"
		{mockPoint: "mock-point", mockValue: "a", headers: [][2]string{{"X-Test-Middleware1", "m1"}, {"X-Test-MockPoint", "a"}}},
		// mock at MockAfterMiddlewares
		{mockPoint: MockAfterMiddlewares, mockValue: "b", headers: [][2]string{{"X-Test-Middleware1", "m1"}, {"X-Test-Middleware2", "m2"}, {"X-Test-MockPoint", "b"}}},
	} {
		if tc.mockPoint != "" {
			RouteMock(tc.mockPoint, func(resp http.ResponseWriter, req *http.Request) {
				resp.Header().Set("X-Test-MockPoint", tc.mockValue)
				resp.WriteHeader(http.StatusOK)
			})
		}
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "http://localhost:8000/foo", nil)
		assert.NoError(t, err)
		r.ServeHTTP(recorder, req)
		assert.Len(t, recorder.Header(), len(tc.headers))
		for _, header := range tc.headers {
			assert.Equal(t, header[1], recorder.Header().Get(header[0]))
		}
		RouteMockReset()
	}
}
