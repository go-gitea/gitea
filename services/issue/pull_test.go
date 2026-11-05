// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"fmt"
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

func TestPullRequestCodeOwnersReviewSkipsReviewersWithoutAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// PR in the private repo org3/repo3
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 12})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.NoError(t, pr.Issue.LoadRepo(t.Context()))
	require.True(t, pr.BaseRepo.IsPrivate)

	member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}) // no access to repo3
	team := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 2})
	teamWithoutRepo := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 12}) // org3 team without repo3

	rules := []*issues_model.CodeOwnerRule{
		{Users: []*user_model.User{member, outsider}, Teams: []*org_model.Team{team, teamWithoutRepo}},
	}

	ctx, committer, err := db.TxContext(t.Context())
	require.NoError(t, err)
	defer committer.Close()

	notifiers, err := pullRequestCodeOwnersReview(ctx, pr, rules)
	require.NoError(t, err)

	var userIDs, teamIDs []int64
	for _, notifier := range notifiers {
		if notifier.Reviewer != nil {
			userIDs = append(userIDs, notifier.Reviewer.ID)
		} else {
			teamIDs = append(teamIDs, notifier.ReviewTeam.ID)
		}
	}
	assert.Equal(t, []int64{member.ID}, userIDs)
	assert.Equal(t, []int64{team.ID}, teamIDs)
	has, err := db.GetEngine(ctx).Exist(&issues_model.Review{IssueID: pr.IssueID, ReviewerID: outsider.ID})
	require.NoError(t, err)
	assert.False(t, has)
	has, err = db.GetEngine(ctx).Exist(&issues_model.Review{IssueID: pr.IssueID, ReviewerTeamID: teamWithoutRepo.ID})
	require.NoError(t, err)
	assert.False(t, has)
}

func TestPullRequestCodeOwnersReviewSkipsOrgsAndForeignTeams(t *testing.T) {
	for _, isPrivate := range []bool{true, false} {
		t.Run(fmt.Sprintf("private=%v", isPrivate), func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())

			// PR in org3/repo3
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 12})
			require.NoError(t, pr.LoadIssue(t.Context()))
			require.NoError(t, pr.LoadBaseRepo(t.Context()))
			require.NoError(t, pr.Issue.LoadRepo(t.Context()))
			pr.BaseRepo.IsPrivate = isPrivate

			member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
			ownerOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})   // org3
			otherOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 6})   // org6
			team := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 2})        // org3 team with repo3
			foreignTeam := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 9}) // org17/review_team

			rules := []*issues_model.CodeOwnerRule{
				{Users: []*user_model.User{member, ownerOrg, otherOrg}, Teams: []*org_model.Team{team, foreignTeam}},
			}

			ctx, committer, err := db.TxContext(t.Context())
			require.NoError(t, err)
			defer committer.Close()

			notifiers, err := pullRequestCodeOwnersReview(ctx, pr, rules)
			require.NoError(t, err)

			var userIDs, teamIDs []int64
			for _, notifier := range notifiers {
				if notifier.Reviewer != nil {
					userIDs = append(userIDs, notifier.Reviewer.ID)
				} else {
					teamIDs = append(teamIDs, notifier.ReviewTeam.ID)
				}
			}
			assert.Equal(t, []int64{member.ID}, userIDs)
			assert.Equal(t, []int64{team.ID}, teamIDs)
		})
	}
}
