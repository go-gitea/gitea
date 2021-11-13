// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/unittestbridge"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func init() {
	unittest.RegisterConsistencyFunc("user", checkForUserConsistency)
	unittest.RegisterConsistencyFunc("repository", checkForRepoConsistency)
	unittest.RegisterConsistencyFunc("issue", checkForIssueConsistency)
	unittest.RegisterConsistencyFunc("pull_request", checkForPullRequestConsistency)
	unittest.RegisterConsistencyFunc("milestone", checkForMilestoneConsistency)
	unittest.RegisterConsistencyFunc("label", checkForLabelConsistency)
	unittest.RegisterConsistencyFunc("team", checkForTeamConsistency)
	unittest.RegisterConsistencyFunc("action", checkForActionConsistency)
}

func checkForUserConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var user = bean.(*User)
	unittest.AssertCountByCond(ta, "repository", builder.Eq{"owner_id": user.ID}, user.NumRepos)
	unittest.AssertCountByCond(ta, "star", builder.Eq{"uid": user.ID}, user.NumStars)
	unittest.AssertCountByCond(ta, "org_user", builder.Eq{"org_id": user.ID}, user.NumMembers)
	unittest.AssertCountByCond(ta, "team", builder.Eq{"org_id": user.ID}, user.NumTeams)
	unittest.AssertCountByCond(ta, "follow", builder.Eq{"user_id": user.ID}, user.NumFollowing)
	unittest.AssertCountByCond(ta, "follow", builder.Eq{"follow_id": user.ID}, user.NumFollowers)
	if user.Type != UserTypeOrganization {
		ta.EqualValues(0, user.NumMembers)
		ta.EqualValues(0, user.NumTeams)
	}
}

func checkForRepoConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var repo = bean.(*Repository)
	ta.Equal(repo.LowerName, strings.ToLower(repo.Name), "repo: %+v", repo)
	unittest.AssertCountByCond(ta, "star", builder.Eq{"repo_id": repo.ID}, repo.NumStars)
	unittest.AssertCountByCond(ta, "milestone", builder.Eq{"repo_id": repo.ID}, repo.NumMilestones)
	unittest.AssertCountByCond(ta, "repository", builder.Eq{"fork_id": repo.ID}, repo.NumForks)
	if repo.IsFork {
		unittest.AssertExistsAndLoadBean(ta, &Repository{ID: repo.ForkID})
	}

	actual := unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "watch", builder.Eq{"repo_id": repo.ID}.
		And(builder.Neq{"mode": RepoWatchModeDont}))
	ta.EqualValues(repo.NumWatches, actual,
		"Unexpected number of watches for repo %+v", repo)

	actual = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.Eq{"is_pull": false, "repo_id": repo.ID})
	ta.EqualValues(repo.NumIssues, actual,
		"Unexpected number of issues for repo %+v", repo)

	actual = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.Eq{"is_pull": false, "is_closed": true, "repo_id": repo.ID})
	ta.EqualValues(repo.NumClosedIssues, actual,
		"Unexpected number of closed issues for repo %+v", repo)

	actual = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.Eq{"is_pull": true, "repo_id": repo.ID})
	ta.EqualValues(repo.NumPulls, actual,
		"Unexpected number of pulls for repo %+v", repo)

	actual = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.Eq{"is_pull": true, "is_closed": true, "repo_id": repo.ID})
	ta.EqualValues(repo.NumClosedPulls, actual,
		"Unexpected number of closed pulls for repo %+v", repo)

	actual = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "milestone", builder.Eq{"is_closed": true, "repo_id": repo.ID})
	ta.EqualValues(repo.NumClosedMilestones, actual,
		"Unexpected number of closed milestones for repo %+v", repo)
}

func checkForIssueConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var issue = bean.(*Issue)
	actual := unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "comment", builder.Eq{"`type`": CommentTypeComment, "issue_id": issue.ID})
	ta.EqualValues(issue.NumComments, actual,
		"Unexpected number of comments for issue %+v", issue)
	if issue.IsPull {
		pr := unittest.AssertExistsAndLoadBean(ta, &PullRequest{IssueID: issue.ID}).(*PullRequest)
		ta.EqualValues(pr.Index, issue.Index)
	}
}

func checkForPullRequestConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var pr = bean.(*PullRequest)
	issue := unittest.AssertExistsAndLoadBean(ta, &Issue{ID: pr.IssueID}).(*Issue)
	ta.True(issue.IsPull)
	ta.EqualValues(issue.Index, pr.Index)
}

func checkForMilestoneConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var milestone = bean.(*Milestone)
	unittest.AssertCountByCond(ta, "issue", builder.Eq{"milestone_id": milestone.ID}, milestone.NumIssues)

	actual := unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.Eq{"is_closed": true, "milestone_id": milestone.ID})
	ta.EqualValues(milestone.NumClosedIssues, actual,
		"Unexpected number of closed issues for milestone %+v", milestone)

	completeness := 0
	if milestone.NumIssues > 0 {
		completeness = milestone.NumClosedIssues * 100 / milestone.NumIssues
	}
	ta.Equal(completeness, milestone.Completeness)
}

func checkForLabelConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var label = bean.(*Label)
	issueLabels := make([]*IssueLabel, 0, 10)
	ta.NoError(db.GetEngine(db.DefaultContext).Find(&issueLabels, &IssueLabel{LabelID: label.ID}))
	ta.EqualValues(label.NumIssues, len(issueLabels),
		"Unexpected number of issue for label %+v", label)

	issueIDs := make([]int64, len(issueLabels))
	for i, issueLabel := range issueLabels {
		issueIDs[i] = issueLabel.IssueID
	}

	expected := int64(0)
	if len(issueIDs) > 0 {
		expected = unittest.GetCountByCond(ta, db.GetEngine(db.DefaultContext), "issue", builder.In("id", issueIDs).And(builder.Eq{"is_closed": true}))
	}
	ta.EqualValues(expected, label.NumClosedIssues,
		"Unexpected number of closed issues for label %+v", label)
}

func checkForTeamConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var team = bean.(*Team)
	unittest.AssertCountByCond(ta, "team_user", builder.Eq{"team_id": team.ID}, team.NumMembers)
	unittest.AssertCountByCond(ta, "team_repo", builder.Eq{"team_id": team.ID}, team.NumRepos)
}

func checkForActionConsistency(ta unittestbridge.Asserter, bean interface{}) {
	var action = bean.(*Action)
	repo := unittest.AssertExistsAndLoadBean(ta, &Repository{ID: action.RepoID}).(*Repository)
	ta.Equal(repo.IsPrivate, action.IsPrivate, "action: %+v", action)
}

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	countBefore, err := db.GetEngine(db.DefaultContext).Count(&PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(db.DefaultContext).Insert(&PullRequest{IssueID: 1000}, &PullRequest{IssueID: 1001}, &PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := CountOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = DeleteOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(db.DefaultContext).Count(&PullRequest{})
	assert.NoError(t, err)
	assert.EqualValues(t, countBefore, countAfter)
}
