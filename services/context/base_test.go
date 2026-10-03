// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	setting.IsInTesting = true
	os.Exit(m.Run())
}

func TestRedirect(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	req.Header.Add("X-Gitea-Fetch-Action", "1")
	b := NewBaseContextForTest(t, resp, req)
	b.Redirect("/other")
	assert.Contains(t, resp.Header().Get("Content-Type"), "application/json")
	assert.JSONEq(t, `{"redirect":"/other"}`, resp.Body.String())
	assert.Equal(t, http.StatusOK, resp.Code)
}
