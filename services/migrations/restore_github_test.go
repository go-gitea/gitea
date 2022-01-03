// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/migration"
	base "code.gitea.io/gitea/modules/migration"
	"github.com/stretchr/testify/assert"
)

func TestParseGithubExportedData(t *testing.T) {
	restorer, err := NewGithubExportedDataRestorer(context.Background(), "../../testdata/github_migration/migration_archive_test_repo.tar.gz", "lunny", "test_repo")
	assert.NoError(t, err)
	assert.EqualValues(t, 49, len(restorer.users))

	repo, err := restorer.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, "test_repo", repo.Name)

	milestones, err := restorer.GetMilestones()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(milestones))

	releases, err := restorer.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(releases))

	labels, err := restorer.GetLabels()
	assert.NoError(t, err)
	assert.EqualValues(t, 9, len(labels))

	issues, isEnd, err := restorer.GetIssues(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(issues))

	comments, isEnd, err := restorer.GetComments(migration.GetCommentOptions{})
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(comments))

	prs, isEnd, err := restorer.GetPullRequests(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(prs))

	reviewers, err := restorer.GetReviews(base.BasicIssueContext(0))
	assert.NoError(t, err)
	assert.EqualValues(t, 6, len(reviewers))
}
