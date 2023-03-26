// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

func TestCanGithubEventMatch(t *testing.T) {
	testCases := []struct {
		desc           string
		eventName      string
		triggeredEvent webhook_module.HookEventType
		expected       bool
	}{
		// registry_package event
		{
			"registry_package matches",
			githubEventRegistryPackage,
			webhook_module.HookEventPackage,
			true,
		},
		{
			"registry_package cannot match",
			githubEventRegistryPackage,
			webhook_module.HookEventPush,
			false,
		},
		// issues event
		{
			"issue matches",
			githubEventIssues,
			webhook_module.HookEventIssueLabel,
			true,
		},
		{
			"issue cannot match",
			githubEventIssues,
			webhook_module.HookEventIssueComment,
			false,
		},
		// issue_comment event
		{
			"issue_comment matches",
			githubEventIssueComment,
			webhook_module.HookEventIssueComment,
			true,
		},
		{
			"issue_comment cannot match",
			githubEventIssueComment,
			webhook_module.HookEventIssues,
			false,
		},
		// pull_request event
		{
			"pull_request matches",
			githubEventPullRequest,
			webhook_module.HookEventPullRequestSync,
			true,
		},
		{
			"pull_request cannot match",
			githubEventPullRequest,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// pull_request_target event
		{
			"pull_request_target matches",
			githubEventPullRequest,
			webhook_module.HookEventPullRequest,
			true,
		},
		{
			"pull_request_target cannot match",
			githubEventPullRequest,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// pull_request_review event
		{
			"pull_request_review matches",
			githubEventPullRequestReview,
			webhook_module.HookEventPullRequestReviewComment,
			true,
		},
		{
			"pull_request_review cannot match",
			githubEventPullRequestReview,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// other events
		{
			"create event",
			githubEventCreate,
			webhook_module.HookEventCreate,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equalf(t, tc.expected, canGithubEventMatch(tc.eventName, tc.triggeredEvent), "canGithubEventMatch(%v, %v)", tc.eventName, tc.triggeredEvent)
		})
	}
}
