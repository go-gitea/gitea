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
			GithubEventRegistryPackage,
			webhook_module.HookEventPackage,
			true,
		},
		{
			"registry_package cannot match",
			GithubEventRegistryPackage,
			webhook_module.HookEventPush,
			false,
		},
		// issues event
		{
			"issue matches",
			GithubEventIssues,
			webhook_module.HookEventIssueLabel,
			true,
		},
		{
			"issue cannot match",
			GithubEventIssues,
			webhook_module.HookEventIssueComment,
			false,
		},
		// issue_comment event
		{
			"issue_comment matches",
			GithubEventIssueComment,
			webhook_module.HookEventIssueComment,
			true,
		},
		{
			"issue_comment cannot match",
			GithubEventIssueComment,
			webhook_module.HookEventIssues,
			false,
		},
		// pull_request event
		{
			"pull_request matches",
			GithubEventPullRequest,
			webhook_module.HookEventPullRequestSync,
			true,
		},
		{
			"pull_request cannot match",
			GithubEventPullRequest,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// pull_request_target event
		{
			"pull_request_target matches",
			GithubEventPullRequest,
			webhook_module.HookEventPullRequest,
			true,
		},
		{
			"pull_request_target cannot match",
			GithubEventPullRequest,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// pull_request_review event
		{
			"pull_request_review matches",
			GithubEventPullRequestReview,
			webhook_module.HookEventPullRequestReviewComment,
			true,
		},
		{
			"pull_request_review cannot match",
			GithubEventPullRequestReview,
			webhook_module.HookEventPullRequestComment,
			false,
		},
		// other events
		{
			"create event",
			GithubEventCreate,
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
