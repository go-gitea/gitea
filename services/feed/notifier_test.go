// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"fmt"
	"strings"
	"testing"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID})
	repo.Owner = user

	oldRepoName := repo.Name
	const newRepoName = "newRepoName"
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	actionBean := &activities_model.Action{
		OpType:    activities_model.ActionRenameRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}
	unittest.AssertNotExistsBean(t, actionBean)

	NewNotifier().RenameRepository(t.Context(), user, repo, oldRepoName)

	unittest.AssertExistsAndLoadBean(t, actionBean)
	unittest.CheckConsistencyFor(t, &activities_model.Action{})
}

func TestNewReleaseActionCreditsDoer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	rel := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 1})
	NewNotifier().NewRelease(t.Context(), doer, rel)

	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{OpType: activities_model.ActionPublishRelease, ActUserID: doer.ID, RepoID: rel.RepoID})
}

func TestPullRequestReviewActionExcerpts(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue.Repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	codeComment := &issues_model.Comment{Type: issues_model.CommentTypeCode, PosterID: doer.ID, IssueID: issue.ID, Content: "```suggestion\n" + strings.Repeat("x", 300) + "\n```\nexplanation [link](https://example.com/" + strings.Repeat("x", 300) + ")"}
	reviewComment := &issues_model.Comment{Type: issues_model.CommentTypeReview, PosterID: doer.ID, IssueID: issue.ID, Content: "![image](image.png)\n\n**summary**\nmore"}
	assert.NoError(t, db.Insert(t.Context(), codeComment, reviewComment))

	NewNotifier().PullRequestReview(t.Context(), nil, &issues_model.Review{
		Type: issues_model.ReviewTypeApprove, Reviewer: doer, ReviewerID: doer.ID, Issue: issue, IssueID: issue.ID,
		CodeComments: issues_model.CodeComments{"file.txt": {1: {codeComment}}},
	}, reviewComment, nil)

	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{OpType: activities_model.ActionCommentPull, CommentID: codeComment.ID, Content: "1|explanation…"})
	unittest.AssertExistsAndLoadBean(t, &activities_model.Action{OpType: activities_model.ActionApprovePullRequest, CommentID: reviewComment.ID, Content: "1|**summary**\nmore"})
}

func TestPushCommits_FeedMaxCommitNum(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.UI.FeedMaxCommitNum, 5)()

	pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// create 7 push commits, with setting.UI.FeedMaxCommitNum = 5
	pushCommits := repository.NewPushCommits()
	pushCommits.Len = 7
	for i := range 7 {
		pushCommits.Commits = append(pushCommits.Commits, &repository.PushCommit{
			Sha1:    fmt.Sprintf("sha%d", i),
			Message: fmt.Sprintf("commit %d", i),
		})
	}
	pushCommits.HeadCommit = pushCommits.Commits[6]
	pushCommits.CompareURL = "compare/sha0...sha6"

	opts := &repository.PushUpdateOptions{
		RefFullName: git.RefName("refs/heads/feed-max-test"),
		OldCommitID: "sha0",
		NewCommitID: "sha6",
	}

	NewNotifier().PushCommits(t.Context(), pusher, repo, opts, pushCommits)

	// Ensure the original pushCommits object was NOT mutated/truncated in place
	assert.Len(t, pushCommits.Commits, 7)
	assert.Equal(t, 7, pushCommits.Len)

	// Verify that the Action stored in the database has truncated commits in JSON Content
	action := unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		OpType:    activities_model.ActionCommitRepo,
		ActUserID: pusher.ID,
		RepoID:    repo.ID,
		RefName:   "refs/heads/feed-max-test",
	})

	var storedCommits repository.PushCommits
	assert.NoError(t, json.Unmarshal([]byte(action.Content), &storedCommits))
	assert.Len(t, storedCommits.Commits, 5)
	assert.Equal(t, 7, storedCommits.Len)

	// Also verify SyncPushCommits does not mutate pushCommits and truncates for feed
	syncOpts := &repository.PushUpdateOptions{
		RefFullName: git.RefName("refs/heads/feed-sync-test"),
		OldCommitID: "sha0",
		NewCommitID: "sha6",
	}

	NewNotifier().SyncPushCommits(t.Context(), pusher, repo, syncOpts, pushCommits)

	assert.Len(t, pushCommits.Commits, 7)
	assert.Equal(t, 7, pushCommits.Len)

	syncAction := unittest.AssertExistsAndLoadBean(t, &activities_model.Action{
		OpType:  activities_model.ActionMirrorSyncPush,
		RepoID:  repo.ID,
		RefName: "refs/heads/feed-sync-test",
	})

	var storedSyncCommits repository.PushCommits
	assert.NoError(t, json.Unmarshal([]byte(syncAction.Content), &storedSyncCommits))
	assert.Len(t, storedSyncCommits.Commits, 5)
	assert.Equal(t, 7, storedSyncCommits.Len)
}
