// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestPayloadSynchronizeBeforeAfter(t *testing.T) {
	payload := &PullRequestPayload{
		Action: HookIssueSynchronized,
		Before: "1111111111111111111111111111111111111111",
		After:  "2222222222222222222222222222222222222222",
		Index:  12,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "synchronized", got["action"])
	assert.Equal(t, "1111111111111111111111111111111111111111", got["before"])
	assert.Equal(t, "2222222222222222222222222222222222222222", got["after"])
	assert.EqualValues(t, 12, got["number"])
}

func TestPullRequestPayloadNonSynchronizeOmitsBeforeAfter(t *testing.T) {
	payload := &PullRequestPayload{
		Action: HookIssueOpened,
		Index:  12,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "opened", got["action"])
	assert.NotContains(t, got, "before")
	assert.NotContains(t, got, "after")
}
