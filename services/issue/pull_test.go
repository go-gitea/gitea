// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"strings"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	org_model "gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestCodeOwnersReview(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 12})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.NoError(t, pr.Issue.LoadRepo(t.Context()))

	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: pr.Issue.PosterID})
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	requestedReviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	team := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 2})

	rules := []*issues_model.CodeOwnerRule{
		{Users: []*user_model.User{poster, reviewer, requestedReviewer}, Teams: []*org_model.Team{team}, ReviewRequest: optional.Some(true)},
		{Users: []*user_model.User{reviewer}, Teams: []*org_model.Team{team}, ReviewRequest: optional.Some(true)},
	}

	for _, tc := range []struct {
		name      string
		reviews   []*issues_model.Review
		wantUsers int
		wantTeams int
	}{
		{"new requests", nil, 1, 1},
		{"member approved", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeApprove}}, 0, 0},
		{"member rejected", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeReject}}, 0, 0},
		{"member commented", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeComment}}, 0, 1},
		{"member requested", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeRequest}}, 0, 1},
		{"non-member approved", []*issues_model.Review{{ReviewerID: 5, Type: issues_model.ReviewTypeApprove}}, 1, 1},
		{"other team reviewed", []*issues_model.Review{{ReviewerTeamID: 1, Type: issues_model.ReviewTypeApprove}}, 1, 1},
		{"member approved then commented", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeApprove}, {ReviewerID: 4, Type: issues_model.ReviewTypeComment}}, 0, 0},
		{"member approval dismissed", []*issues_model.Review{{ReviewerID: 4, Type: issues_model.ReviewTypeApprove, Dismissed: true}}, 1, 1},
		{"team already requested", []*issues_model.Review{{ReviewerTeamID: 2, Type: issues_model.ReviewTypeRequest}}, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, committer, err := db.TxContext(t.Context())
			require.NoError(t, err)
			defer committer.Close()

			for _, review := range tc.reviews {
				review.IssueID = pr.IssueID
				require.NoError(t, db.Insert(ctx, review))
			}
			before, err := db.GetEngine(ctx).Count(&issues_model.Review{IssueID: pr.IssueID})
			require.NoError(t, err)

			disabled := []*issues_model.CodeOwnerRule{{Users: []*user_model.User{reviewer}, Teams: []*org_model.Team{team}, ReviewRequest: optional.Some(false)}}
			notifiers, err := pullRequestCodeOwnersReview(ctx, pr, disabled)
			require.NoError(t, err)
			assert.Empty(t, notifiers)

			notifiers, err = pullRequestCodeOwnersReview(ctx, pr, rules)
			require.NoError(t, err)

			var users, teams int
			for _, notifier := range notifiers {
				assert.True(t, notifier.IsAdd)
				require.NotNil(t, notifier.Comment)
				require.NotNil(t, notifier.Comment.CommentMetaData)
				assert.Equal(t, issues_model.SpecialDoerNameCodeOwners, notifier.Comment.CommentMetaData.SpecialDoerName)
				if notifier.Reviewer != nil {
					users++
					assert.Equal(t, reviewer, notifier.Reviewer)
					assert.Nil(t, notifier.ReviewTeam)
					assert.Equal(t, reviewer.ID, notifier.Comment.AssigneeID)
				} else {
					teams++
					assert.Equal(t, team, notifier.ReviewTeam)
					assert.Equal(t, team.ID, notifier.Comment.AssigneeTeamID)
				}
			}
			assert.Equal(t, tc.wantUsers, users)
			assert.Equal(t, tc.wantTeams, teams)

			notifiers, err = pullRequestCodeOwnersReview(ctx, pr, rules)
			require.NoError(t, err)
			assert.Empty(t, notifiers)
			after, err := db.GetEngine(ctx).Count(&issues_model.Review{IssueID: pr.IssueID})
			require.NoError(t, err)
			assert.Equal(t, before+int64(tc.wantUsers+tc.wantTeams), after)
		})
	}
}

func TestMatchCodeOwnerRules(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	rules, warnings := issues_model.GetCodeOwnersFromContent(t.Context(), strings.TrimSpace(`
*.md @user1
[Docs] @user5
*.md
README.md @user4
[Docs] @user2
other.md
[Independent] @user1
*.md
`))
	require.Empty(t, warnings)
	for _, tc := range []struct {
		paths    []string
		lineNums []int
	}{
		{[]string{"README.md"}, []int{1, 4, 8}},
		{[]string{"README.md", "guide.md"}, []int{1, 3, 4, 8}},
		{[]string{"other.md"}, []int{1, 6, 8}},
		{[]string{"file.go"}, nil},
	} {
		t.Run(strings.Join(tc.paths, ","), func(t *testing.T) {
			matching, warnings := MatchCodeOwnerRules(rules, tc.paths)
			assert.Empty(t, warnings)
			var lineNums []int
			for _, rule := range matching {
				lineNums = append(lineNums, rule.LineNum)
			}
			assert.Equal(t, tc.lineNums, lineNums)
		})
	}
}

func TestPullRequestCodeOwnersReviewWatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 12})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.NoError(t, pr.Issue.LoadRepo(t.Context()))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	for _, tc := range []struct {
		name                string
		mode                repo_model.WatchMode
		includePullRequests bool
		content             string
		want                bool
	}{
		{"not watching", repo_model.WatchModeNone, true, "*.md @user4", false},
		{"ignoring", repo_model.WatchModeDont, true, "*.md @user4", false},
		{"watching pull requests", repo_model.WatchModeCustom, true, "*.md @user4", true},
		{"not watching pull requests", repo_model.WatchModeCustom, false, "*.md @user4", false},
		{"explicit true without watch", repo_model.WatchModeNone, false, "*.md @user4 ## review-request=true", true},
		{"explicit true overrides opt out", repo_model.WatchModeCustom, false, "*.md @user4 ## review-request=true", true},
		{"explicit false overrides watch", repo_model.WatchModeCustom, true, "*.md @user4 ## review-request=false", false},
		{"explicit true on another rule", repo_model.WatchModeCustom, false, "*.md @user4\n*.js @user4 ## review-request=true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, committer, err := db.TxContext(t.Context())
			require.NoError(t, err)
			defer committer.Close()
			require.NoError(t, repo_model.WatchRepoWithOptions(ctx, reviewer, pr.BaseRepo, repo_model.WatchOptions{
				Mode: tc.mode, WatchPullRequests: tc.includePullRequests, WatchIssues: true,
			}))
			rules, warnings := issues_model.GetCodeOwnersFromContent(ctx, tc.content)
			require.Empty(t, warnings)
			notifiers, err := pullRequestCodeOwnersReview(ctx, pr, rules)
			require.NoError(t, err)
			if tc.want {
				require.Len(t, notifiers, 1)
				assert.Equal(t, reviewer.ID, notifiers[0].Reviewer.ID)
			} else {
				assert.Empty(t, notifiers)
			}
		})
	}
}
