// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCORSNotSet(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestf(t, "GET", "/api/v1/version")
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, resp.Code, http.StatusOK)
	corsHeader := resp.Header().Get("Access-Control-Allow-Origin")
	assert.Empty(t, corsHeader, "Access-Control-Allow-Origin: generated header should match") // header not set
}
