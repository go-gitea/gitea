// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssueTemplateList(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})

		// no issue template
		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/issue_templates")
		resp := MakeRequest(t, req, http.StatusOK)
		issueTemplates := DecodeJSON(t, resp, []*api.IssueTemplate{})
		assert.Empty(t, issueTemplates)

		// one correct issue template and some incorrect issue templates
		err := createOrReplaceFileInBranch(user, repo, ".gitea/ISSUE_TEMPLATE/tmpl-ok.md", repo.DefaultBranch, `----
name: foo
about: bar
----
`)
		assert.NoError(t, err)

		err = createOrReplaceFileInBranch(user, repo, ".gitea/ISSUE_TEMPLATE/tmpl-err1.yml", repo.DefaultBranch, `name: '`)
		assert.NoError(t, err)

		err = createOrReplaceFileInBranch(user, repo, ".gitea/ISSUE_TEMPLATE/tmpl-err2.yml", repo.DefaultBranch, `other: `)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/issue_templates")
		resp = MakeRequest(t, req, http.StatusOK)
		issueTemplates = DecodeJSON(t, resp, []*api.IssueTemplate{})
		assert.Len(t, issueTemplates, 1)
		assert.Equal(t, "foo", issueTemplates[0].Name)
		assert.Equal(t, "error occurs when parsing issue template: count=2", resp.Header().Get("X-Gitea-Warning"))
	})
}

func TestAPIIssueTemplateRequiresCodeUnit(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 24})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadRepository)
	issueTemplatesURL := "/api/v1/repos/" + repo.FullName() + "/issue_templates"
	languagesURL := "/api/v1/repos/" + repo.FullName() + "/languages"

	req := NewRequest(t, "GET", issueTemplatesURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	req = NewRequest(t, "GET", languagesURL).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}
