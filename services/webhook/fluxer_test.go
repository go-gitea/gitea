// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFluxerJSONPayload(t *testing.T) {
	data, err := pushTestPayload().JSONPayload()
	require.NoError(t, err)
	hook := &webhook_model.Webhook{Type: webhook_module.FLUXER, URL: "https://chat.example/api/webhooks/123/token?wait=true&custom=value", Meta: `{"username":"Gitea","icon_url":"https://gitea.example/icon.png"}`}
	task := &webhook_model.HookTask{EventType: webhook_module.HookEventPush, PayloadContent: string(data), PayloadVersion: 2}
	req, body, err := newFluxerRequest(t.Context(), hook, task)
	require.NoError(t, err)
	defer req.Body.Close()
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, hook.URL, req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	received, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, body, received)
	var payload FluxerPayload
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload.Embeds, 1)
	assert.Equal(t, "Gitea", payload.Username)
	assert.Equal(t, "https://gitea.example/icon.png", payload.AvatarURL)
	assert.Equal(t, "[test/repo:test] 2 new commits", payload.Embeds[0].Title)
	assert.Contains(t, payload.Embeds[0].Description, "[2020558](http://localhost:3000/test/repo/commit/")
	assert.NotNil(t, payload.AllowedMentions.Parse)
	assert.Empty(t, payload.AllowedMentions.Parse)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(body, &wire))
	for _, field := range []string{"wait", "content", "tts", "avatar"} {
		assert.NotContains(t, wire, field)
	}
	assert.Equal(t, map[string]any{"parse": []any{}}, wire["allowed_mentions"])
}

func TestFluxerPayload(t *testing.T) {
	review := pullRequestTestPayload()
	review.Action = api.HookIssueReviewed
	deleted := repositoryTestPayload()
	deleted.Action = api.HookRepoDeleted
	issue := issueTestPayload()
	issue.Action = api.HookIssueOpened
	wiki := wikiTestPayload()
	wiki.Action = api.HookWikiCreated
	sender := &api.User{UserName: "user1", AvatarURL: "https://gitea.example/avatar.png"}
	cases := []struct {
		event   webhook_module.HookEventType
		payload api.Payloader
	}{
		{webhook_module.HookEventCreate, createTestPayload()},
		{webhook_module.HookEventDelete, deleteTestPayload()},
		{webhook_module.HookEventFork, forkTestPayload()},
		{webhook_module.HookEventPush, pushTestPayload()},
		{webhook_module.HookEventIssues, issue},
		{webhook_module.HookEventIssueComment, issueCommentTestPayload()},
		{webhook_module.HookEventPullRequestComment, pullRequestCommentTestPayload()},
		{webhook_module.HookEventPullRequest, pullRequestTestPayload()},
		{webhook_module.HookEventPullRequestReviewApproved, review},
		{webhook_module.HookEventPullRequestReviewRejected, review},
		{webhook_module.HookEventPullRequestReviewComment, review},
		{webhook_module.HookEventRepository, deleted},
		{webhook_module.HookEventRelease, pullReleaseTestPayload()},
		{webhook_module.HookEventWiki, wiki},
		{webhook_module.HookEventPackage, packageTestPayload()},
		{webhook_module.HookEventStatus, &api.CommitStatusPayload{Sender: sender, SHA: strings.Repeat("a", 40), Context: "ci", Description: "Passed", State: "success"}},
		{webhook_module.HookEventWorkflowRun, &api.WorkflowRunPayload{Sender: sender, Action: "completed", WorkflowRun: &api.ActionWorkflowRun{DisplayTitle: "Build", Conclusion: "success", HTMLURL: "https://gitea.example/actions/runs/1"}}},
		{webhook_module.HookEventWorkflowJob, &api.WorkflowJobPayload{Sender: sender, Action: "completed", WorkflowJob: &api.ActionWorkflowJob{Name: "Test", Conclusion: "success", HTMLURL: "https://gitea.example/actions/runs/1/jobs/1"}}},
	}
	for _, tc := range cases {
		t.Run(string(tc.event), func(t *testing.T) {
			data, err := tc.payload.JSONPayload()
			require.NoError(t, err)
			source, err := newPayload(discordConvertor{}, data, tc.event)
			require.NoError(t, err)
			req, body, err := newFluxerRequest(t.Context(), &webhook_model.Webhook{URL: "https://chat.example/webhook"}, &webhook_model.HookTask{EventType: tc.event, PayloadContent: string(data), PayloadVersion: 2})
			require.NoError(t, err)
			require.NoError(t, req.Body.Close())
			var payload FluxerPayload
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Len(t, payload.Embeds, 1)
			assert.NotEmpty(t, payload.Embeds[0].Title)
			assert.Equal(t, source.Embeds[0].Title, payload.Embeds[0].Title)
			assert.Equal(t, source.Embeds[0].Description, payload.Embeds[0].Description)
			assert.Equal(t, source.Embeds[0].Color, payload.Embeds[0].Color)
			assert.Equal(t, source.Embeds[0].Author.Name, payload.Embeds[0].Author.Name)
			assert.NotContains(t, string(body), `"footer"`)
			assert.NotContains(t, string(body), `"fields"`)
			if tc.event == webhook_module.HookEventRepository || tc.event == webhook_module.HookEventStatus {
				assert.Empty(t, payload.Embeds[0].URL)
			}
		})
	}
}

func TestFluxerPayloadOptionalFields(t *testing.T) {
	payload := fluxerPayloadFromDiscord(DiscordPayload{Embeds: []DiscordEmbed{{Title: "Deleted", URL: "", Author: DiscordEmbedAuthor{Name: " \u202e\f "}}}})
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var wire struct {
		Embeds []map[string]any `json:"embeds"`
	}
	require.NoError(t, json.Unmarshal(data, &wire))
	require.Len(t, wire.Embeds, 1)
	for _, field := range []string{"footer", "fields", "author", "url", "description"} {
		assert.NotContains(t, wire.Embeds[0], field)
	}
	payload = fluxerPayloadFromDiscord(DiscordPayload{Embeds: []DiscordEmbed{{Author: DiscordEmbedAuthor{Name: "A", URL: "invalid", IconURL: ""}}}})
	data, err = json.Marshal(payload)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"url"`)
	assert.NotContains(t, string(data), `"icon_url"`)
}

func TestFluxerPayloadLimits(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{strings.Repeat("a", 256), strings.Repeat("a", 256)},
		{strings.Repeat("a", 257), strings.Repeat("a", 256)},
		{strings.Repeat("😀", 129), strings.Repeat("😀", 128)},
		{strings.Repeat("a", 255) + "😀", strings.Repeat("a", 255)},
		{"\ufeff\f\u202e Title \ufeff", "Title"},
		{"\f\u202e\u00a0", ""},
	} {
		got := truncateFluxerString(tc.input, 256)
		assert.Equal(t, tc.want, got)
		assert.True(t, utf8.ValidString(got))
		assert.LessOrEqual(t, len(utf16.Encode([]rune(got))), 256)
	}
	source := discordConvertor{}.createPayload(&api.User{UserName: strings.Repeat("😀", 129)}, strings.Repeat("😀", 129), strings.Repeat("😀", 3000), "", greenColor)
	payload := fluxerPayloadFromDiscord(source)
	assert.Equal(t, strings.Repeat("😀", 128), payload.Embeds[0].Title)
	assert.Equal(t, strings.Repeat("😀", 128), payload.Embeds[0].Author.Name)
	assert.Equal(t, strings.Repeat("😀", 2000), payload.Embeds[0].Description)
}

func TestFluxerURL(t *testing.T) {
	base := "https://gitea.example/"
	for _, tc := range []struct {
		url   string
		valid bool
	}{
		{base, true},
		{base + "?q=a#fragment", true},
		{"http://127.0.0.1:3000/a", true},
		{"https://[::1]:3000/a", true},
		{"https://nørd.example/avatar", true},
		{"https://例え.テスト/avatar", true},
		{base + strings.Repeat("a", 2048-len(base)), true},
		{base + strings.Repeat("a", 2049-len(base)), false},
		{"", false},
		{"https://localhost:3000/a", false},
		{"https://gitea.example./a", false},
		{"https://user:pass@gitea.example/a", false},
		{"ftp://gitea.example/a", false},
		{"https://gitea.example:0/a", false},
		{"https://gitea.example:65536/a", false},
		{"HTTPS://gitea.example/a", false},
		{"https://ＦＯＯ.example/a", false},
		{"https://foo。example/a", false},
		{"https://foo.😀/a", false},
		{"https://foo.例/a", false},
		{"https://" + strings.Repeat("ø", 64) + ".example/a", false},
		{"https://bad_host.example/a", false},
		{"https://gitea.123/a", false},
		{"https://gitea.example/a b", false},
		{"https://gitea.example/<x>", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			if tc.valid {
				assert.Equal(t, tc.url, fluxerURL(tc.url))
			} else {
				assert.Empty(t, fluxerURL(tc.url))
			}
		})
	}
}

func TestFluxerMeta(t *testing.T) {
	assert.True(t, IsValidHookTaskType(webhook_module.FLUXER))
	for _, meta := range []string{"", `{}`, `{"username":"","icon_url":""}`} {
		got, err := GetFluxerHook(&webhook_model.Webhook{Meta: meta})
		require.NoError(t, err)
		assert.Equal(t, &FluxerMeta{}, got)
	}
	got, err := GetFluxerHook(&webhook_model.Webhook{Meta: `{"username":" \u202eGitea ","icon_url":" https://gitea.example/icon.png "}`})
	require.NoError(t, err)
	assert.Equal(t, &FluxerMeta{Username: "Gitea", IconURL: "https://gitea.example/icon.png"}, got)
	for _, meta := range []string{`{`, `{"username":123}`, `{"username":"` + strings.Repeat("😀", 41) + `"}`, `{"icon_url":"https://localhost/icon.png"}`} {
		_, err := GetFluxerHook(&webhook_model.Webhook{Meta: meta})
		require.Error(t, err)
	}
	valid := &FluxerMeta{Username: strings.Repeat("😀", 40)}
	require.NoError(t, valid.Validate())
}
