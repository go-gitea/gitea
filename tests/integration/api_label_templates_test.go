// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.dev/modules/options"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListLabelTemplates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("list labels", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/label/templates")
		resp := MakeRequest(t, req, http.StatusOK)

		templateList := DecodeJSON(t, resp, []string{})

		for i := range repo_module.LabelTemplateFiles {
			assert.Equal(t, repo_module.LabelTemplateFiles[i].DisplayName, templateList[i])
		}
	})

	t.Run("label info", func(t *testing.T) {
		// If Gitea has for some reason no Label templates, we need to skip this test
		if len(repo_module.LabelTemplateFiles) == 0 {
			return
		}

		// Use the first template for the test
		templateName := repo_module.LabelTemplateFiles[0].DisplayName

		urlStr := "/api/v1/label/templates/" + url.PathEscape(templateName)
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		templateInfo := DecodeJSON(t, resp, []api.LabelTemplate{})

		labels, err := repo_module.LoadTemplateLabelsByDisplayName(templateName)
		assert.NoError(t, err)

		for i := range labels {
			assert.Equal(t, strings.TrimLeft(labels[i].Color, "#"), templateInfo[i].Color)
			assert.Equal(t, labels[i].Description, templateInfo[i].Description)
			assert.Equal(t, labels[i].Exclusive, templateInfo[i].Exclusive)
			assert.Equal(t, labels[i].Name, templateInfo[i].Name)
		}
	})

	t.Run("list licenses", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/licenses")
		resp := MakeRequest(t, req, http.StatusOK)

		licenseList := DecodeJSON(t, resp, []api.LicensesTemplateListEntry{})
		assert.Contains(t, licenseList, api.LicensesTemplateListEntry{
			Key:  "MIT",
			Name: "MIT",
			URL:  setting.AppURL + "api/v1/licenses/MIT",
		})
	})

	t.Run("license info", func(t *testing.T) {
		// If Gitea has for some reason no License templates, we need to skip this test
		if len(repo_module.Licenses) == 0 {
			return
		}

		// Use the first template for the test
		licenseName := repo_module.Licenses[0]

		urlStr := "/api/v1/licenses/" + url.PathEscape(licenseName)
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		licenseInfo := DecodeJSON(t, resp, &api.LicenseTemplateInfo{})

		// We get the text of the template here
		text, _ := options.License(licenseName)

		assert.Equal(t, licenseInfo.Key, licenseName)
		assert.Equal(t, licenseInfo.Name, licenseName)
		assert.Equal(t, licenseInfo.Body, string(text))
	})
}
