// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"testing"

	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestParseGithubExportedData(t *testing.T) {
	restorer, err := NewGithubExportedDataRestorer(context.Background(), "../../testdata/github_migration/migration_archive_test_repo.tar.gz", "lunny", "test_repo")
	assert.NoError(t, err)
	assert.EqualValues(t, 49, len(restorer.users))

	// repo info
	repo, err := restorer.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, "test_repo", repo.Name)

	// milestones
	milestones, err := restorer.GetMilestones()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(milestones))

	// releases
	releases, err := restorer.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(releases))
	assert.EqualValues(t, 0, len(releases[0].Assets))

	// labels
	labels, err := restorer.GetLabels()
	assert.NoError(t, err)
	assert.EqualValues(t, 9, len(labels))

	// issues
	issues, isEnd, err := restorer.GetIssues(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(issues))
	assert.EqualValues(t, 1, issues[0].Context.ForeignID())
	assert.EqualValues(t, "Please add an animated gif icon to the merge button", issues[0].Title)
	assert.EqualValues(t, "I just want the merge button to hurt my eyes a little. 😝 ", issues[0].Content)
	assert.EqualValues(t, "guillep2k", issues[0].PosterName)
	assert.EqualValues(t, 2, len(issues[0].Labels), fmt.Sprintf("%#v", issues[0].Labels))
	assert.EqualValues(t, "1.0.0", issues[0].Milestone)
	assert.EqualValues(t, 0, len(issues[0].Assets))
	assert.EqualValues(t, 1, len(issues[0].Reactions))
	assert.EqualValues(t, "closed", issues[0].State)
	assert.NotNil(t, issues[0].Closed)
	assert.NotZero(t, issues[0].Updated)
	assert.NotZero(t, issues[0].Created)

	assert.EqualValues(t, 2, issues[1].Context.ForeignID())
	assert.EqualValues(t, "Test issue", issues[1].Title)
	assert.EqualValues(t, "This is test issue 2, do not touch!", issues[1].Content)
	assert.EqualValues(t, "mrsdizzie", issues[1].PosterName)
	assert.EqualValues(t, 1, len(issues[1].Labels))
	assert.EqualValues(t, "1.1.0", issues[1].Milestone)
	assert.EqualValues(t, 0, len(issues[1].Assets))
	assert.EqualValues(t, 6, len(issues[1].Reactions))
	assert.EqualValues(t, "closed", issues[1].State)
	assert.NotNil(t, issues[1].Closed)
	assert.NotZero(t, issues[1].Updated)
	assert.NotZero(t, issues[1].Created)

	// comments
	comments, isEnd, err := restorer.GetComments(base.GetCommentOptions{})
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 16, len(comments))
	// first comments are comment type
	assert.EqualValues(t, 2, comments[0].IssueIndex)
	assert.NotZero(t, comments[0].Created)

	assert.EqualValues(t, 2, comments[1].IssueIndex)
	assert.NotZero(t, comments[1].Created)

	// pull requests
	prs, isEnd, err := restorer.GetPullRequests(1, 100)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 2, len(prs))

	assert.EqualValues(t, "Update README.md", prs[0].Title)
	assert.EqualValues(t, "add warning to readme", prs[0].Content)
	assert.EqualValues(t, 1, len(prs[0].Labels))
	assert.EqualValues(t, "documentation", prs[0].Labels[0].Name)

	assert.EqualValues(t, "Test branch", prs[1].Title)
	assert.EqualValues(t, "do not merge this PR", prs[1].Content)
	assert.EqualValues(t, 1, len(prs[1].Labels))
	assert.EqualValues(t, "bug", prs[1].Labels[0].Name)

	// reviews
	reviews, isEnd, err := restorer.GetReviews(base.GetReviewOptions{})
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.EqualValues(t, 6, len(reviews))
}
