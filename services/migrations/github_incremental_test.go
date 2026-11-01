// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGraphQLIncrementalQueryOrder(t *testing.T) {
	g := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{}}
	assert.Contains(t, g.graphQLIssuesQuery(), "orderBy:{field:CREATED_AT,direction:ASC}")
	assert.NotContains(t, g.graphQLIssuesQuery(), "$since")

	g.since = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	issues := g.graphQLIssuesQuery()
	assert.Contains(t, issues, "$since:DateTime!")
	assert.Contains(t, issues, "orderBy:{field:UPDATED_AT,direction:ASC},filterBy:{since:$since}")
	prs := g.graphQLPullRequestsQuery()
	assert.Contains(t, prs, "orderBy:{field:UPDATED_AT,direction:DESC}")
	assert.NotContains(t, prs, "$since") // pullRequests has no since filter
}

func TestOnlyGitHubSupportsSyncing(t *testing.T) {
	assert.True(t, (&GithubDownloaderV3{}).SupportSyncing())
	assert.False(t, (&GitBucketDownloader{GithubDownloaderV3: &GithubDownloaderV3{}}).SupportSyncing())
}
