// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

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
