// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/options"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListLicenseTemplates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/licenses")
	resp := MakeRequest(t, req, http.StatusOK)

	// This tests if the API returns a list of strings
	var licenseList []api.LicensesTemplateListEntry
	DecodeJSON(t, resp, &licenseList)
}

func TestAPIGetLicenseTemplateInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// If Gitea has for some reason no License templates, we need to skip this test
	if len(repo_module.Licenses) == 0 {
		return
	}

	// Use the first template for the test
	licenseName := repo_module.Licenses[0]

	urlStr := fmt.Sprintf("/api/v1/licenses/%s", url.PathEscape(licenseName))
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var licenseInfo api.LicenseTemplateInfo
	DecodeJSON(t, resp, &licenseInfo)

	// We get the text of the template here
	text, _ := options.License(licenseName)

	assert.Equal(t, licenseInfo.Key, licenseName)
	assert.Equal(t, licenseInfo.Name, licenseName)
	assert.Equal(t, licenseInfo.Body, string(text))
}
