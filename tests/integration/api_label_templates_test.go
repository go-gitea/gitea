// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListLabelTemplates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/label/templates")
	resp := MakeRequest(t, req, http.StatusOK)

	var templateList []string
	DecodeJSON(t, resp, &templateList)

	for i := range repo_module.LabelTemplateFiles {
		assert.Equal(t, repo_module.LabelTemplateFiles[i].DisplayName, templateList[i])
	}
}

func TestAPIGetLabelTemplateInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// If Gitea has for some reason no Label templates, we need to skip this test
	if len(repo_module.LabelTemplateFiles) == 0 {
		return
	}

	// Use the first template for the test
	templateName := repo_module.LabelTemplateFiles[0].DisplayName

	urlStr := fmt.Sprintf("/api/v1/label/templates/%s", url.PathEscape(templateName))
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var templateInfo []api.LabelTemplate
	DecodeJSON(t, resp, &templateInfo)

	labels, err := repo_module.LoadTemplateLabelsByDisplayName(templateName)
	assert.NoError(t, err)

	for i := range labels {
		assert.Equal(t, strings.TrimLeft(labels[i].Color, "#"), templateInfo[i].Color)
		assert.Equal(t, labels[i].Description, templateInfo[i].Description)
		assert.Equal(t, labels[i].Exclusive, templateInfo[i].Exclusive)
		assert.Equal(t, labels[i].Name, templateInfo[i].Name)
	}
}
