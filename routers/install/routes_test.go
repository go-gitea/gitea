// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRoutes(t *testing.T) {
	r := Routes()
	assert.NotNil(t, r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `class="page-content install"`)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/no-such", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/assets/img/gitea.svg", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
