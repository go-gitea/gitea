// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestPayload(t *testing.T) {
	for _, tt := range []struct {
		payload PullRequestPayload
		want    string
	}{
		{
			payload: PullRequestPayload{
				Action: HookIssueSynchronized,
				Before: "1111111111111111111111111111111111111111",
				After:  "2222222222222222222222222222222222222222",
				Index:  12,
			},
			want: `
		"action": "synchronized",
		"before": "1111111111111111111111111111111111111111",
		"after": "2222222222222222222222222222222222222222",`,
		},
		{
			payload: PullRequestPayload{Action: HookIssueOpened, Index: 12},
			want: `
		"action": "opened",`,
		},
	} {
		t.Run(string(tt.payload.Action), func(t *testing.T) {
			data, err := json.Marshal(&tt.payload)
			require.NoError(t, err)
			assert.JSONEq(t, `{`+tt.want+`
		"number": 12,
		"commit_id": "",
		"pull_request": null,
		"repository": null,
		"requested_reviewer": null,
		"review": null,
		"sender": null
	}`, string(data))
		})
	}
}
