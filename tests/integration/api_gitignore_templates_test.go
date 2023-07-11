// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/options"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListGitignoresTemplates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/gitignore/templates")
	resp := MakeRequest(t, req, http.StatusOK)

	// This tests if the API returns a list of strings
	var gitignoreList []string
	DecodeJSON(t, resp, &gitignoreList)
}

func TestAPIGetGitignoreTemplateInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// If Gitea has for some reason no Gitignore templates, we need to skip this test
	if len(repo_module.Gitignores) == 0 {
		return
	}

	// Use the first template for the test
	templateName := repo_module.Gitignores[0]

	urlStr := fmt.Sprintf("/api/v1/gitignore/templates/%s", templateName)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var templateInfo api.GitignoreTemplateInfo
	DecodeJSON(t, resp, &templateInfo)

	// We get the text of the template here
	text, _ := options.Gitignore(templateName)

	assert.Equal(t, templateInfo.Name, templateName)
	assert.Equal(t, templateInfo.Source, string(text))
}
