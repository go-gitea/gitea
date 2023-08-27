// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testLanguageList(t *testing.T, uri string) {
	req := NewRequest(t, "GET", uri)
	resp := MakeRequest(t, req, http.StatusOK)

	var langs []api.LanguageInfo
	DecodeJSON(t, resp, &langs)

	for _, lang := range langs {
		assert.NotEqual(t, lang.Name, "")
		assert.NotEqual(t, lang.Color, "")
	}
}

func TestAPIListLanguages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testLanguageList(t, "/api/v1/repos/languages")
}

func TestAPIListUserLanguages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	loginUser(t, "user2")
	testLanguageList(t, "/api/v1/users/user2/languages")
}

func TestAPIListOrgLanguages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	loginUser(t, "user2")
	testLanguageList(t, "/api/v1/orgs/user3/languages")
}