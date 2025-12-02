// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPullCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user5")
	req := NewRequest(t, "GET", "/user2/repo1/pulls/3/commits/list")
	resp := session.MakeRequest(t, req, http.StatusOK)

	var pullCommitList struct {
		Commits             []pull_service.CommitInfo `json:"commits"`
		LastReviewCommitSha string                    `json:"last_review_commit_sha"`
	}
	DecodeJSON(t, resp, &pullCommitList)

	require.Len(t, pullCommitList.Commits, 2)
	assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", pullCommitList.Commits[0].ID)
	assert.Equal(t, "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2", pullCommitList.Commits[1].ID)
	assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", pullCommitList.LastReviewCommitSha)

	t.Run("CommitBlobExcerpt", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req = NewRequest(t, "GET", "/user2/repo1/blob_excerpt/985f0301dba5e7b34be866819cd15ad3d8f508ee?last_left=0&last_right=0&left=2&right=2&left_hunk_size=2&right_hunk_size=2&path=README.md&style=split&direction=up")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), `<td class="lines-code lines-code-new"><code class="code-inner"># repo1</code>`)
	})
}
