// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

func TestDetectMatched(t *testing.T) {
	testCases := []struct {
		desc         string
		commit       *git.Commit
		triggedEvent webhook_module.HookEventType
		payload      api.Payloader
		yamlOn       string
		expected     bool
	}{
		{
			desc:         "HookEventCreate(create) matches GithubEventCreate(create)",
			triggedEvent: webhook_module.HookEventCreate,
			payload:      nil,
			yamlOn:       "on: create",
			expected:     true,
		},
		{
			desc:         "HookEventIssues(issues) `opened` action matches GithubEventIssues(issues)",
			triggedEvent: webhook_module.HookEventIssues,
			payload:      &api.IssuePayload{Action: api.HookIssueOpened},
			yamlOn:       "on: issues",
			expected:     true,
		},
		{
			desc:         "HookEventIssues(issues) `milestoned` action matches GithubEventIssues(issues)",
			triggedEvent: webhook_module.HookEventIssues,
			payload:      &api.IssuePayload{Action: api.HookIssueMilestoned},
			yamlOn:       "on: issues",
			expected:     true,
		},
		{
			desc:         "HookEventPullRequestSync(pull_request_sync) matches GithubEventPullRequest(pull_request)",
			triggedEvent: webhook_module.HookEventPullRequestSync,
			payload:      &api.PullRequestPayload{Action: api.HookIssueSynchronized},
			yamlOn:       "on: pull_request",
			expected:     true,
		},
		{
			desc:         "HookEventPullRequest(pull_request) `label_updated` action doesn't match GithubEventPullRequest(pull_request) with no activity type",
			triggedEvent: webhook_module.HookEventPullRequest,
			payload:      &api.PullRequestPayload{Action: api.HookIssueLabelUpdated},
			yamlOn:       "on: pull_request",
			expected:     false,
		},
		{
			desc:         "HookEventPullRequest(pull_request) `closed` action doesn't match GithubEventPullRequest(pull_request) with no activity type",
			triggedEvent: webhook_module.HookEventPullRequest,
			payload:      &api.PullRequestPayload{Action: api.HookIssueClosed},
			yamlOn:       "on: pull_request",
			expected:     false,
		},
		{
			desc:         "HookEventPullRequest(pull_request) `closed` action doesn't match GithubEventPullRequest(pull_request) with branches",
			triggedEvent: webhook_module.HookEventPullRequest,
			payload: &api.PullRequestPayload{
				Action: api.HookIssueClosed,
				PullRequest: &api.PullRequest{
					Base: &api.PRBranchInfo{},
				},
			},
			yamlOn:   "on:\n  pull_request:\n    branches: [main]",
			expected: false,
		},
		{
			desc:         "HookEventPullRequest(pull_request) `label_updated` action matches GithubEventPullRequest(pull_request) with `label` activity type",
			triggedEvent: webhook_module.HookEventPullRequest,
			payload:      &api.PullRequestPayload{Action: api.HookIssueLabelUpdated},
			yamlOn:       "on:\n  pull_request:\n    types: [labeled]",
			expected:     true,
		},
		{
			desc:         "HookEventPullRequestReviewComment(pull_request_review_comment) matches GithubEventPullRequestReviewComment(pull_request_review_comment)",
			triggedEvent: webhook_module.HookEventPullRequestReviewComment,
			payload:      &api.PullRequestPayload{Action: api.HookIssueReviewed},
			yamlOn:       "on:\n  pull_request_review_comment:\n    types: [created]",
			expected:     true,
		},
		{
			desc:         "HookEventPullRequestReviewRejected(pull_request_review_rejected) doesn't match GithubEventPullRequestReview(pull_request_review) with `dismissed` activity type (we don't support `dismissed` at present)",
			triggedEvent: webhook_module.HookEventPullRequestReviewRejected,
			payload:      &api.PullRequestPayload{Action: api.HookIssueReviewed},
			yamlOn:       "on:\n  pull_request_review:\n    types: [dismissed]",
			expected:     false,
		},
		{
			desc:         "HookEventRelease(release) `published` action matches GithubEventRelease(release) with `published` activity type",
			triggedEvent: webhook_module.HookEventRelease,
			payload:      &api.ReleasePayload{Action: api.HookReleasePublished},
			yamlOn:       "on:\n  release:\n    types: [published]",
			expected:     true,
		},
		{
			desc:         "HookEventPackage(package) `created` action doesn't match GithubEventRegistryPackage(registry_package) with `updated` activity type",
			triggedEvent: webhook_module.HookEventPackage,
			payload:      &api.PackagePayload{Action: api.HookPackageCreated},
			yamlOn:       "on:\n  registry_package:\n    types: [updated]",
			expected:     false,
		},
		{
			desc:         "HookEventWiki(wiki) matches GithubEventGollum(gollum)",
			triggedEvent: webhook_module.HookEventWiki,
			payload:      nil,
			yamlOn:       "on: gollum",
			expected:     true,
		},
		{
			desc:         "HookEventSchedue(schedule) matches GithubEventSchedule(schedule)",
			triggedEvent: webhook_module.HookEventSchedule,
			payload:      nil,
			yamlOn:       "on: schedule",
			expected:     true,
		},
		{
			desc:         "push to tag matches workflow with paths condition (should skip paths check)",
			triggedEvent: webhook_module.HookEventPush,
			payload: &api.PushPayload{
				Ref:    "refs/tags/v1.0.0",
				Before: "0000000",
				Commits: []*api.PayloadCommit{
					{
						ID:      "abcdef123456",
						Added:   []string{"src/main.go"},
						Message: "Release v1.0.0",
					},
				},
			},
			commit:   nil,
			yamlOn:   "on:\n  push:\n    paths:\n      - src/**",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			evts, err := GetEventsFromContent([]byte(tc.yamlOn))
			assert.NoError(t, err)
			assert.Len(t, evts, 1)
			assert.Equal(t, tc.expected, detectMatched(nil, tc.commit, tc.triggedEvent, tc.payload, evts[0]))
		})
	}
}
