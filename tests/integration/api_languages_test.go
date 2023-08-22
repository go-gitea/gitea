// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"
)

func TestAPIListLanguages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/repos/languages")
	resp := MakeRequest(t, req, http.StatusOK)

	var langs map[string]string
	DecodeJSON(t, resp, &langs)
}
