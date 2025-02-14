// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagistPayload(t *testing.T) {
	pc := packagistConvertor{
		PackageURL: "https://packagist.org/packages/example",
	}
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := pc.Create(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := pc.Delete(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := pc.Fork(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := pc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "https://packagist.org/packages/example", pl.PackagistRepository.URL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := pc.Issue(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)

		p.Action = api.HookIssueClosed
		pl, err = pc.Issue(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := pc.IssueComment(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := pc.PullRequest(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := pc.IssueComment(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := pc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := pc.Repository(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := pc.Package(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := pc.Wiki(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)

		p.Action = api.HookWikiEdited
		pl, err = pc.Wiki(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)

		p.Action = api.HookWikiDeleted
		pl, err = pc.Wiki(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := pc.Release(p)
		require.NoError(t, err)
		require.Equal(t, PackagistPayload{}, pl)
	})
}

func TestPackagistJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
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
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newPackagistRequest(context.Background(), hook, task)
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
	assert.Equal(t, "https://packagist.org/packages/example", body.PackagistRepository.URL)
}

func TestPackagistEmptyPayload(t *testing.T) {
	p := createTestPayload()
	data, err := p.JSONPayload()
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
		EventType:      webhook_module.HookEventCreate,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newPackagistRequest(context.Background(), hook, task)
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
	assert.Equal(t, "", body.PackagistRepository.URL)
}
