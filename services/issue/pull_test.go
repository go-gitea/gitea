// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	org_model "gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

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
		{Users: []*user_model.User{poster, reviewer, requestedReviewer}, Teams: []*org_model.Team{team}},
		{Users: []*user_model.User{reviewer}, Teams: []*org_model.Team{team}},
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

			notifiers, err := pullRequestCodeOwnersReview(ctx, pr, rules)
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
