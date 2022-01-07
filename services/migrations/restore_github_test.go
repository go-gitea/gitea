// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/migration"

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
	assert.EqualValues(t, 0, len(releases[0].Assets))

	labels, err := restorer.GetLabels()
	assert.NoError(t, err)
	assert.EqualValues(t, 9, len(labels))

	issues, isEnd, err := restorer.GetIssues(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(issues))
	assert.EqualValues(t, 1, issues[0].Context.ForeignID())
	assert.EqualValues(t, "Please add an animated gif icon to the merge button", issues[0].Title)
	assert.EqualValues(t, "I just want the merge button to hurt my eyes a little. 😝 ", issues[0].Content)
	assert.EqualValues(t, "justusbunsi", issues[0].PosterName)
	assert.EqualValues(t, 1, len(issues[0].Labels))
	assert.EqualValues(t, "1.1.0", len(issues[0].Milestone))
	assert.EqualValues(t, 0, len(issues[0].Assets))
	assert.EqualValues(t, 1, len(issues[0].Reactions))
	assert.True(t, issues[0].State == "closed")

	assert.EqualValues(t, 2, issues[1].Context.ForeignID())
	assert.EqualValues(t, "Please add an animated gif icon to the merge button", issues[1].Title)
	assert.EqualValues(t, "I just want the merge button to hurt my eyes a little. 😝", issues[1].Content)
	assert.EqualValues(t, "guillep2k", issues[1].PosterName)
	assert.EqualValues(t, 2, len(issues[1].Labels))
	assert.EqualValues(t, "1.0.0", len(issues[1].Milestone))
	assert.EqualValues(t, 0, len(issues[1].Assets))
	assert.EqualValues(t, 6, len(issues[1].Reactions))
	assert.True(t, issues[1].State == "closed")

	comments, isEnd, err := restorer.GetComments(migration.GetCommentOptions{})
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(comments))

	prs, isEnd, err := restorer.GetPullRequests(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(prs))

	reviewers, err := restorer.GetReviews(migration.BasicIssueContext(0))
	assert.NoError(t, err)
	assert.EqualValues(t, 6, len(reviewers))
}
