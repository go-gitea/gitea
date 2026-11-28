// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsPayloadRoundTrip(t *testing.T) {
	payload := &api.PullRequestPayload{
		Action: api.HookIssueOpened,
		Index:  42,
		PullRequest: &api.PullRequest{
			HTMLURL: "https://example.com/pr/42",
		},
	}

	payloadType, payloadJSON, err := marshalActionsPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, actionsPayloadTypePullRequest, payloadType)

	decoded, err := unmarshalActionsPayload(payloadType, payloadJSON)
	require.NoError(t, err)

	decodedPayload, ok := decoded.(*api.PullRequestPayload)
	require.True(t, ok)
	assert.Equal(t, payload.Action, decodedPayload.Action)
	assert.Equal(t, payload.Index, decodedPayload.Index)
	assert.Equal(t, payload.PullRequest.HTMLURL, decodedPayload.PullRequest.HTMLURL)
}

func TestActionsPayloadRoundTripNil(t *testing.T) {
	payloadType, payloadJSON, err := marshalActionsPayload(nil)
	require.NoError(t, err)
	assert.Empty(t, payloadType)
	assert.Nil(t, payloadJSON)

	decoded, err := unmarshalActionsPayload(payloadType, payloadJSON)
	require.NoError(t, err)
	assert.Nil(t, decoded)
}

func TestActionsNotifyQueueToNotifyInputLoadsPullRequestIssue(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.BaseRepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	input, err := (&actionsNotifyQueueItem{
		Method:        "NewPullRequest",
		RepoID:        repo.ID,
		DoerID:        doer.ID,
		Event:         "pull_request",
		PullRequestID: pr.ID,
	}).toNotifyInput(t.Context())
	require.NoError(t, err)
	require.NotNil(t, input.PullRequest)
	require.NotNil(t, input.PullRequest.Issue)
	assert.Equal(t, pr.IssueID, input.PullRequest.Issue.ID)
	assert.Equal(t, input.PullRequest.GetGitHeadRefName(), input.Ref.String())
}
