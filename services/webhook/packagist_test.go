// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagistPayload(t *testing.T) {
	pc := packagistConvertor{
		PackageURL: "https://packagist.org/packages/example",
	}
	for _, testCase := range []struct {
		name string
		call func() (PackagistPayload, error)
	}{
		{"Create", func() (PackagistPayload, error) { return pc.Create(createTestPayload()) }},
		{"Delete", func() (PackagistPayload, error) { return pc.Delete(deleteTestPayload()) }},
		{"Fork", func() (PackagistPayload, error) { return pc.Fork(forkTestPayload()) }},
		{"IssueComment", func() (PackagistPayload, error) { return pc.IssueComment(issueCommentTestPayload()) }},
		{"PullRequest", func() (PackagistPayload, error) { return pc.PullRequest(pullRequestTestPayload()) }},
		{"PullRequestComment", func() (PackagistPayload, error) { return pc.IssueComment(pullRequestCommentTestPayload()) }},
		{"Review", func() (PackagistPayload, error) {
			p := pullRequestTestPayload()
			p.Action = api.HookIssueReviewed
			return pc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		}},
		{"Repository", func() (PackagistPayload, error) { return pc.Repository(repositoryTestPayload()) }},
		{"Package", func() (PackagistPayload, error) { return pc.Package(packageTestPayload()) }},
		{"Release", func() (PackagistPayload, error) { return pc.Release(pullReleaseTestPayload()) }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			pl, err := testCase.call()
			require.NoError(t, err)
			require.Equal(t, PackagistPayload{}, pl)
		})
	}

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := pc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "https://packagist.org/packages/example", pl.PackagistRepository.URL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()
		for _, action := range []api.HookIssueAction{api.HookIssueOpened, api.HookIssueClosed} {
			p.Action = action
			pl, err := pc.Issue(p)
			require.NoError(t, err)
			require.Equal(t, PackagistPayload{}, pl)
		}
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()
		for _, action := range []api.HookWikiAction{api.HookWikiCreated, api.HookWikiEdited, api.HookWikiDeleted} {
			p.Action = action
			pl, err := pc.Wiki(p)
			require.NoError(t, err)
			require.Equal(t, PackagistPayload{}, pl)
		}
	})
}

func TestPackagistJSONPayload(t *testing.T) {
	for _, testCase := range []struct {
		event   webhook_module.HookEventType
		payload func() api.Payloader
	}{
		{webhook_module.HookEventPush, func() api.Payloader { return pushTestPayload() }},
		{webhook_module.HookEventCreate, func() api.Payloader { return createTestPayload() }},
	} {
		t.Run(string(testCase.event), func(t *testing.T) {
			data, err := testCase.payload().JSONPayload()
			require.NoError(t, err)

			hook := &webhook_model.Webhook{
				RepoID:     3,
				IsActive:   true,
				Type:       webhook_module.PACKAGIST,
				URL:        "https://packagist.org/api/update-package?username=THEUSERNAME&apiToken=TOPSECRETAPITOKEN",
				Meta:       `{"package_url":"https://packagist.org/packages/example"}`,
				HTTPMethod: "POST",
			}
			task := &webhook_model.HookTask{
				HookID:         hook.ID,
				EventType:      testCase.event,
				PayloadContent: string(data),
				PayloadVersion: 2,
			}

			req, reqBody, err := newPackagistRequest(t.Context(), hook, task)
			require.NotNil(t, req)
			require.NotNil(t, reqBody)
			require.NoError(t, err)

			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, "https://packagist.org/api/update-package?username=THEUSERNAME&apiToken=TOPSECRETAPITOKEN", req.URL.String())
			assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			var body PackagistPayload
			err = json.NewDecoder(req.Body).Decode(&body)
			assert.NoError(t, err)
			if testCase.event == webhook_module.HookEventPush {
				assert.Equal(t, "https://packagist.org/packages/example", body.PackagistRepository.URL)
			} else {
				assert.Empty(t, body.PackagistRepository.URL)
			}
		})
	}
}
