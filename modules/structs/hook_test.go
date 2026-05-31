// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"gitea.dev/modules/json"

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

	assert.JSONEq(t, `{
		"action": "synchronized",
		"before": "1111111111111111111111111111111111111111",
		"after": "2222222222222222222222222222222222222222",
		"number": 12,
		"commit_id": "",
		"pull_request": null,
		"repository": null,
		"requested_reviewer": null,
		"review": null,
		"sender": null
	}`, string(data))
}

func TestPullRequestPayloadNonSynchronizeOmitsBeforeAfter(t *testing.T) {
	payload := &PullRequestPayload{
		Action: HookIssueOpened,
		Index:  12,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"action": "opened",
		"number": 12,
		"commit_id": "",
		"pull_request": null,
		"repository": null,
		"requested_reviewer": null,
		"review": null,
		"sender": null
	}`, string(data))
}
