// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestListPullCommits(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user5")
		req := NewRequest(t, "GET", "/user2/repo1/pulls/3/commits/list")
		resp := session.MakeRequest(t, req, http.StatusOK)

		var pullCommitList struct {
			Commits             []pull_service.CommitInfo `json:"commits"`
			LastReviewCommitSha string                    `json:"last_review_commit_sha"`
		}
		DecodeJSON(t, resp, &pullCommitList)

		if assert.Len(t, pullCommitList.Commits, 2) {
			assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", pullCommitList.Commits[0].ID)
			assert.Equal(t, "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2", pullCommitList.Commits[1].ID)
		}
		assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", pullCommitList.LastReviewCommitSha)
	})
}
