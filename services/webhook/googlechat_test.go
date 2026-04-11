// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleChatPayload(t *testing.T) {
	iconURL := "gitea-notification-icon"
	gc := googleChatConvertor{Name: "test", IconURL: iconURL}

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := gc.Push(p)
		require.NoError(t, err)

		assert.Empty(t, pl.Text)
		require.Len(t, pl.CardsV2, 1)
		assert.Equal(t, "gitea-notification", pl.CardsV2[0].CardID)
		assert.Equal(t, "test", pl.CardsV2[0].Card.Header.Title)
		assert.Equal(t, "Gitea Webhook", pl.CardsV2[0].Card.Header.Subtitle)
		assert.Equal(t, iconURL, pl.CardsV2[0].Card.Header.ImageURL)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, fmt.Sprintf("[%s:%s] 2 new commits pushed by %s",
			GoogleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName),
			googleChatLinkToRef(p.Repo.HTMLURL, p.Ref),
			googleChatUserLink(p.Pusher),
		), widgets[0].TextParagraph.Text)
		commit := p.Commits[0]
		commitLink := GoogleChatLinkFormatter(commit.URL, commit.ID[:7])
		commitMessage := GoogleChatTextFormatter(strings.TrimRight(strings.SplitN(commit.Message, "\n", 2)[0], "\r"))
		commitText := fmt.Sprintf("%s: %s - %s", commitLink, commitMessage, GoogleChatTextFormatter(commit.Author.Name))
		assert.Equal(t, fmt.Sprintf("%s\n%s", commitText, commitText), widgets[1].TextParagraph.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := gc.PullRequest(p)
		require.NoError(t, err)

		require.Len(t, pl.CardsV2, 1)
		assert.Equal(t, "test", pl.CardsV2[0].Card.Header.Title)
		assert.Equal(t, "Gitea Webhook", pl.CardsV2[0].Card.Header.Subtitle)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, fmt.Sprintf("[%s] Pull request opened: %s by %s",
			GoogleChatLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName),
			GoogleChatLinkFormatter(p.PullRequest.URL, fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)),
			googleChatUserLink(p.Sender),
		), widgets[0].TextParagraph.Text)
		assert.Equal(t, GoogleChatTextFormatter(p.PullRequest.Body), widgets[1].TextParagraph.Text)
	})
}

func TestGoogleChatJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	iconURL := "gitea-notification-icon"
	hookURL := (&url.URL{
		Scheme: "https",
		Host:   "chat.googleapis.com",
		Path:   "/v1/spaces/example/messages",
		RawQuery: url.Values{
			"key":   []string{"key"},
			"token": []string{"token"},
		}.Encode(),
	}).String()
	hook := &webhook_model.Webhook{
		RepoID:     3,
		Name:       "test",
		IsActive:   true,
		Type:       webhook_module.GOOGLECHAT,
		URL:        hookURL,
		Meta:       fmt.Sprintf(`{"icon_url":%q}`, iconURL),
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newGoogleChatRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, hook.URL, req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var raw map[string]any
	require.NoError(t, json.Unmarshal(reqBody, &raw))
	assert.NotContains(t, raw, "text")

	var body GoogleChatPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Empty(t, body.Text)
	require.Len(t, body.CardsV2, 1)
	assert.Equal(t, iconURL, body.CardsV2[0].Card.Header.ImageURL)
	assert.Equal(t, "test", body.CardsV2[0].Card.Header.Title)
	assert.Equal(t, "Gitea Webhook", body.CardsV2[0].Card.Header.Subtitle)
	require.Len(t, body.CardsV2[0].Card.Sections[0].Widgets, 2)
}
