// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestNewWebHookLink(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	baseurl := "/user2/repo1/settings/hooks"
	tests := []string{
		// webhook list page
		baseurl,
		// new webhook page
		baseurl + "/gitea/new",
		// edit webhook page
		baseurl + "/1",
	}

	for _, url := range tests {
		resp := session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		menus := htmlDoc.doc.Find(".ui.top.attached.header .ui.dropdown .menu a")
		menus.Each(func(i int, menu *goquery.Selection) {
			url, exist := menu.Attr("href")
			assert.True(t, exist)
			assert.True(t, strings.HasPrefix(url, baseurl))
		})
	}
}

func testAPICreateWebhookForRepo(t *testing.T, session *TestSession, userName, repoName, url, event string) {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+userName+"/"+repoName+"/hooks", api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          url,
		},
		Events: []string{event},
		Active: true,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func testCreateWebhookForRepo(t *testing.T, session *TestSession, webhookType, userName, repoName, url, eventKind string) {
	csrf := GetUserCSRFToken(t, session)
	req := NewRequestWithValues(t, "POST", "/"+userName+"/"+repoName+"/settings/hooks/"+webhookType+"/new", map[string]string{
		"_csrf":        csrf,
		"payload_url":  url,
		"events":       eventKind,
		"active":       "true",
		"content_type": fmt.Sprintf("%d", webhook.ContentTypeJSON),
		"http_method":  "POST",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func testAPICreateWebhookForOrg(t *testing.T, session *TestSession, userName, url, event string) {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/"+userName+"/hooks", api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          url,
		},
		Events: []string{event},
		Active: true,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

type mockWebhookProvider struct {
	server *httptest.Server
}

func newMockWebhookProvider(callback func(r *http.Request), status int) *mockWebhookProvider {
	m := &mockWebhookProvider{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callback(r)
		w.WriteHeader(status)
	}))
	return m
}

func (m *mockWebhookProvider) URL() string {
	if m.server == nil {
		return ""
	}
	return m.server.URL
}

// Close closes the mock webhook http server
func (m *mockWebhookProvider) Close() {
	if m.server != nil {
		m.server.Close()
		m.server = nil
	}
}

func Test_WebhookCreate(t *testing.T) {
	var payloads []api.CreatePayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.CreatePayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = string(webhook_module.HookEventCreate)
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "create")

		// 2. trigger the webhook
		testAPICreateBranch(t, session, "user2", "repo1", "master", "master2", http.StatusCreated)

		// 3. validate the webhook is triggered
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, string(webhook_module.HookEventCreate), triggeredEvent)
		assert.EqualValues(t, "repo1", payloads[0].Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repo.FullName)
		assert.EqualValues(t, "master2", payloads[0].Ref)
		assert.EqualValues(t, "branch", payloads[0].RefType)
	})
}

func Test_WebhookDelete(t *testing.T) {
	var payloads []api.DeletePayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.DeletePayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "delete"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "delete")

		// 2. trigger the webhook
		testAPICreateBranch(t, session, "user2", "repo1", "master", "master2", http.StatusCreated)
		testAPIDeleteBranch(t, "master2", http.StatusNoContent)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "delete", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "repo1", payloads[0].Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repo.FullName)
		assert.EqualValues(t, "master2", payloads[0].Ref)
		assert.EqualValues(t, "branch", payloads[0].RefType)
	})
}

func Test_WebhookFork(t *testing.T) {
	var payloads []api.ForkPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.ForkPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "fork"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user1")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "fork")

		// 2. trigger the webhook
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1-fork", "master")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "fork", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "repo1-fork", payloads[0].Repo.Name)
		assert.EqualValues(t, "user1/repo1-fork", payloads[0].Repo.FullName)
		assert.EqualValues(t, "repo1", payloads[0].Forkee.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Forkee.FullName)
	})
}

func Test_WebhookIssueComment(t *testing.T) {
	var payloads []api.IssueCommentPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.IssueCommentPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "issue_comment"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "issue_comment")

		// 2. trigger the webhook
		issueURL := testNewIssue(t, session, "user2", "repo1", "Title2", "Description2")
		testIssueAddComment(t, session, issueURL, "issue title2 comment1", "")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "issue_comment", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "created", payloads[0].Action)
		assert.EqualValues(t, "repo1", payloads[0].Issue.Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Issue.Repo.FullName)
		assert.EqualValues(t, "Title2", payloads[0].Issue.Title)
		assert.EqualValues(t, "Description2", payloads[0].Issue.Body)
		assert.EqualValues(t, "issue title2 comment1", payloads[0].Comment.Body)
	})
}

func Test_WebhookRelease(t *testing.T) {
	var payloads []api.ReleasePayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.ReleasePayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "release"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "release")

		// 2. trigger the webhook
		createNewRelease(t, session, "/user2/repo1", "v0.0.99", "v0.0.99", false, false)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "release", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "repo1", payloads[0].Repository.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repository.FullName)
		assert.EqualValues(t, "v0.0.99", payloads[0].Release.TagName)
		assert.False(t, payloads[0].Release.IsDraft)
		assert.False(t, payloads[0].Release.IsPrerelease)
	})
}

func Test_WebhookPush(t *testing.T) {
	var payloads []api.PushPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.PushPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "push"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "push")

		// 2. trigger the webhook
		testCreateFile(t, session, "user2", "repo1", "master", "test_webhook_push.md", "# a test file for webhook push")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "push", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "repo1", payloads[0].Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repo.FullName)
		assert.Len(t, payloads[0].Commits, 1)
		assert.EqualValues(t, []string{"test_webhook_push.md"}, payloads[0].Commits[0].Added)
	})
}

func Test_WebhookIssue(t *testing.T) {
	var payloads []api.IssuePayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.IssuePayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "issues"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "issues")

		// 2. trigger the webhook
		testNewIssue(t, session, "user2", "repo1", "Title1", "Description1")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "issues", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "opened", payloads[0].Action)
		assert.EqualValues(t, "repo1", payloads[0].Issue.Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Issue.Repo.FullName)
		assert.EqualValues(t, "Title1", payloads[0].Issue.Title)
		assert.EqualValues(t, "Description1", payloads[0].Issue.Body)
	})
}

func Test_WebhookPullRequest(t *testing.T) {
	var payloads []api.PullRequestPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.PullRequestPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "pull_request"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "pull_request")

		testAPICreateBranch(t, session, "user2", "repo1", "master", "master2", http.StatusCreated)
		// 2. trigger the webhook
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1})
		testCreatePullToDefaultBranch(t, session, repo1, repo1, "master2", "first pull request")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "pull_request", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "repo1", payloads[0].PullRequest.Base.Repository.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].PullRequest.Base.Repository.FullName)
		assert.EqualValues(t, "repo1", payloads[0].PullRequest.Head.Repository.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].PullRequest.Head.Repository.FullName)
		assert.EqualValues(t, 0, payloads[0].PullRequest.Additions)
	})
}

func Test_WebhookPullRequestComment(t *testing.T) {
	var payloads []api.IssueCommentPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.IssueCommentPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "pull_request_comment"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "pull_request_comment")

		// 2. trigger the webhook
		testAPICreateBranch(t, session, "user2", "repo1", "master", "master2", http.StatusCreated)
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1})
		prID := testCreatePullToDefaultBranch(t, session, repo1, repo1, "master2", "first pull request")

		testIssueAddComment(t, session, "/user2/repo1/pulls/"+prID, "pull title2 comment1", "")

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "pull_request_comment", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "created", payloads[0].Action)
		assert.EqualValues(t, "repo1", payloads[0].Issue.Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Issue.Repo.FullName)
		assert.EqualValues(t, "first pull request", payloads[0].Issue.Title)
		assert.EqualValues(t, "", payloads[0].Issue.Body)
		assert.EqualValues(t, "pull title2 comment1", payloads[0].Comment.Body)
	})
}

func Test_WebhookWiki(t *testing.T) {
	var payloads []api.WikiPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.WikiPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "wiki"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "wiki")

		// 2. trigger the webhook
		testAPICreateWikiPage(t, session, "user2", "repo1", "Test Wiki Page", http.StatusCreated)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "wiki", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "created", payloads[0].Action)
		assert.EqualValues(t, "repo1", payloads[0].Repository.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repository.FullName)
		assert.EqualValues(t, "Test-Wiki-Page", payloads[0].Page)
	})
}

func Test_WebhookRepository(t *testing.T) {
	var payloads []api.RepositoryPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.RepositoryPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "repository"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user1")

		testAPICreateWebhookForOrg(t, session, "org3", provider.URL(), "repository")

		// 2. trigger the webhook
		testAPIOrgCreateRepo(t, session, "org3", "repo_new", http.StatusCreated)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "repository", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "created", payloads[0].Action)
		assert.EqualValues(t, "org3", payloads[0].Organization.UserName)
		assert.EqualValues(t, "repo_new", payloads[0].Repository.Name)
		assert.EqualValues(t, "org3/repo_new", payloads[0].Repository.FullName)
	})
}

func Test_WebhookPackage(t *testing.T) {
	var payloads []api.PackagePayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		var payload api.PackagePayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "package"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user1")

		testAPICreateWebhookForOrg(t, session, "org3", provider.URL(), "package")

		// 2. trigger the webhook
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)
		url := fmt.Sprintf("/api/packages/%s/generic/%s/%s", "org3", "gitea", "v1.24.0")
		req := NewRequestWithBody(t, "PUT", url+"/gitea", strings.NewReader("This is a dummy file")).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "package", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, "created", payloads[0].Action)
		assert.EqualValues(t, "gitea", payloads[0].Package.Name)
		assert.EqualValues(t, "generic", payloads[0].Package.Type)
		assert.EqualValues(t, "org3", payloads[0].Organization.UserName)
		assert.EqualValues(t, "v1.24.0", payloads[0].Package.Version)
	})
}

func Test_WebhookStatus(t *testing.T) {
	var payloads []api.CommitStatusPayload
	var triggeredEvent string
	provider := newMockWebhookProvider(func(r *http.Request) {
		assert.Contains(t, r.Header["X-Github-Event-Type"], "status", "X-GitHub-Event-Type should contain status")
		assert.Contains(t, r.Header["X-Gitea-Event-Type"], "status", "X-Gitea-Event-Type should contain status")
		assert.Contains(t, r.Header["X-Gogs-Event-Type"], "status", "X-Gogs-Event-Type should contain status")
		assert.Contains(t, r.Header["X-Github-Event"], "status", "X-GitHub-Event should contain status")
		assert.Contains(t, r.Header["X-Gitea-Event"], "status", "X-Gitea-Event should contain status")
		assert.Contains(t, r.Header["X-Gogs-Event"], "status", "X-Gogs-Event should contain status")
		content, _ := io.ReadAll(r.Body)
		var payload api.CommitStatusPayload
		err := json.Unmarshal(content, &payload)
		assert.NoError(t, err)
		payloads = append(payloads, payload)
		triggeredEvent = "status"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "status")

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1})

		gitRepo1, err := gitrepo.OpenRepository(context.Background(), repo1)
		assert.NoError(t, err)
		commitID, err := gitRepo1.GetBranchCommitID(repo1.DefaultBranch)
		assert.NoError(t, err)

		// 2. trigger the webhook
		testCtx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeAll)

		// update a status for a commit via API
		doAPICreateCommitStatus(testCtx, commitID, api.CreateStatusOption{
			State:       api.CommitStatusSuccess,
			TargetURL:   "http://test.ci/",
			Description: "",
			Context:     "testci",
		})(t)

		// 3. validate the webhook is triggered
		assert.EqualValues(t, "status", triggeredEvent)
		assert.Len(t, payloads, 1)
		assert.EqualValues(t, commitID, payloads[0].Commit.ID)
		assert.EqualValues(t, "repo1", payloads[0].Repo.Name)
		assert.EqualValues(t, "user2/repo1", payloads[0].Repo.FullName)
		assert.EqualValues(t, "testci", payloads[0].Context)
		assert.EqualValues(t, commitID, payloads[0].SHA)
	})
}

func Test_WebhookStatus_NoWrongTrigger(t *testing.T) {
	var trigger string
	provider := newMockWebhookProvider(func(r *http.Request) {
		assert.NotContains(t, r.Header["X-Github-Event-Type"], "status", "X-GitHub-Event-Type should not contain status")
		assert.NotContains(t, r.Header["X-Gitea-Event-Type"], "status", "X-Gitea-Event-Type should not contain status")
		assert.NotContains(t, r.Header["X-Gogs-Event-Type"], "status", "X-Gogs-Event-Type should not contain status")
		trigger = "push"
	}, http.StatusOK)
	defer provider.Close()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// 1. create a new webhook with special webhook for repo1
		session := loginUser(t, "user2")

		// create a push_only webhook from web UI
		testCreateWebhookForRepo(t, session, "gitea", "user2", "repo1", provider.URL(), "push_only")

		// 2. trigger the webhook with a push action
		testCreateFile(t, session, "user2", "repo1", "master", "test_webhook_push.md", "# a test file for webhook push")

		// 3. validate the webhook is triggered with right event
		assert.EqualValues(t, "push", trigger)
	})
}
