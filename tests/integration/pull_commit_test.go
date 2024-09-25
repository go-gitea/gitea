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
			assert.Equal(t, "5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", pullCommitList.Commits[0].ID)
			assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", pullCommitList.Commits[1].ID)
		}
		assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", pullCommitList.LastReviewCommitSha)
	})
}
