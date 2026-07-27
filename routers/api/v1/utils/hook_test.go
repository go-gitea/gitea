// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestTestHookValidation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("Test Validation", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "gitea",
			Config: map[string]string{
				"content_type": "json",
				"url":          "https://example.com/webhook",
			},
		})
		assert.Equal(t, 0, ctx.Resp.WrittenStatus()) // not written yet
	})

	t.Run("Test Validation with invalid URL", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "gitea",
			Config: map[string]string{
				"content_type": "json",
				"url":          "example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})

	t.Run("Test Validation with invalid webhook type", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "unknown",
			Config: map[string]string{
				"content_type": "json",
				"url":          "example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})

	t.Run("Test Validation with empty content type", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "unknown",
			Config: map[string]string{
				"url": "https://example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})
}

func TestUpdateHookEvents(t *testing.T) {
	t.Run("pull_request enables all PR-related events", func(t *testing.T) {
		events := updateHookEvents([]string{"pull_request"})
		assert.True(t, events[webhook_module.HookEventPullRequest])
		assert.True(t, events[webhook_module.HookEventPullRequestAssign])
		assert.True(t, events[webhook_module.HookEventPullRequestLabel])
		assert.True(t, events[webhook_module.HookEventPullRequestMilestone])
		assert.True(t, events[webhook_module.HookEventPullRequestComment])
		assert.True(t, events[webhook_module.HookEventPullRequestReview])
		assert.True(t, events[webhook_module.HookEventPullRequestReviewRequest])
		assert.True(t, events[webhook_module.HookEventPullRequestSync])
		assert.False(t, events[webhook_module.HookEventPush])
		assert.False(t, events[webhook_module.HookEventIssues])
	})

	t.Run("pull_request_only enables only main pull_request event", func(t *testing.T) {
		events := updateHookEvents([]string{"pull_request_only"})
		assert.True(t, events[webhook_module.HookEventPullRequest])
		assert.False(t, events[webhook_module.HookEventPullRequestAssign])
		assert.False(t, events[webhook_module.HookEventPullRequestLabel])
		assert.False(t, events[webhook_module.HookEventPullRequestReview])
		assert.False(t, events[webhook_module.HookEventPullRequestSync])
	})

	t.Run("issues enables all issue-related events", func(t *testing.T) {
		events := updateHookEvents([]string{"issues"})
		assert.True(t, events[webhook_module.HookEventIssues])
		assert.True(t, events[webhook_module.HookEventIssueAssign])
		assert.True(t, events[webhook_module.HookEventIssueLabel])
		assert.True(t, events[webhook_module.HookEventIssueMilestone])
		assert.True(t, events[webhook_module.HookEventIssueComment])
		assert.False(t, events[webhook_module.HookEventPullRequest])
	})

	t.Run("issues_only enables only main issues event", func(t *testing.T) {
		events := updateHookEvents([]string{"issues_only"})
		assert.True(t, events[webhook_module.HookEventIssues])
		assert.False(t, events[webhook_module.HookEventIssueAssign])
		assert.False(t, events[webhook_module.HookEventIssueComment])
	})

	t.Run("empty defaults to push", func(t *testing.T) {
		events := updateHookEvents(nil)
		assert.True(t, events[webhook_module.HookEventPush])
		assert.False(t, events[webhook_module.HookEventPullRequest])
	})
}

func TestHookEventsRoundTrip(t *testing.T) {
	// EventsArray output must be accepted by updateHookEvents and restore the same flags.
	cases := [][]string{
		{"pull_request"},
		{"pull_request_only"},
		{"issues"},
		{"issues_only"},
		{"pull_request_only", "pull_request_assign", "push"},
		{"issues_only", "issue_comment"},
		{"push", "create", "release"},
	}
	for _, input := range cases {
		t.Run(strings.Join(input, "+"), func(t *testing.T) {
			stored := updateHookEvents(input)
			w := &webhook.Webhook{
				HookEvent: &webhook_module.HookEvent{
					ChooseEvents: true,
					HookEvents:   stored,
				},
			}
			// Only compare events that updateHookEvents can express
			got := updateHookEvents(w.EventsArray())
			assert.Equal(t, stored, got, "EventsArray %v did not round-trip", w.EventsArray())
		})
	}
}
