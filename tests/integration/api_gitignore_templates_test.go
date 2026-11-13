// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/modules/options"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGitignoreTemplates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("List", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/gitignore/templates")
		resp := MakeRequest(t, req, http.StatusOK)

		templateList := DecodeJSON(t, resp, []string{}) // this is a very long list
		assert.Contains(t, templateList, "C++")
		assert.Contains(t, templateList, "Go")
	})

	t.Run("Info", func(t *testing.T) {
		// If Gitea has for some reason no Gitignore templates, we need to skip this test
		if len(repo_module.Gitignores) == 0 {
			return
		}

		// Use the first template for the test
		templateName := repo_module.Gitignores[0]

		urlStr := "/api/v1/gitignore/templates/" + templateName
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		templateInfo := DecodeJSON(t, resp, &api.GitignoreTemplateInfo{})

		// We get the text of the template here
		text, _ := options.Gitignore(templateName)

		assert.Equal(t, templateInfo.Name, templateName)
		assert.Equal(t, templateInfo.Source, string(text))
	})
}
