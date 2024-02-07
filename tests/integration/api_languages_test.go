// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http"
	"net/url"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func testLanguageList(t *testing.T, uri string, exp []map[string]string) {
	req := NewRequest(t, "GET", uri)
	resp := MakeRequest(t, req, http.StatusOK)

	var langs []api.LanguageInfo
	DecodeJSON(t, resp, &langs)

	assert.Equal(t, len(langs), len(exp))

	for i, lang := range langs {
		assert.Equal(t, lang.Name, exp[i]["name"])
		assert.Equal(t, lang.Color, exp[i]["color"])
	}
}

func TestAPIListLanguages(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testLanguageList(t,
			"/api/v1/repos/languages",
			[]map[string]string{
				{
					"name":  "Markdown",
					"color": "#083fa1",
				},
				{
					"name":  "Text",
					"color": "#cccccc",
				},
			},
		)
	})
}

func TestAPIListUserLanguages(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testLanguageList(t,
			"/api/v1/users/user2/languages",
			[]map[string]string{
				{
					"name":  "Text",
					"color": "#cccccc",
				},
			},
		)
	})
}

func TestAPIListOrgLanguages(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testLanguageList(t,
			"/api/v1/orgs/user3/languages",
			[]map[string]string{},
		)
	})
}
