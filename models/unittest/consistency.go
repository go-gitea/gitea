// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittest

import (
	"reflect"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

const (
	// these const values are copied from `models` package to prevent from cycle-import
	modelsUserTypeOrganization = 1
	modelsRepoWatchModeDont    = 2
	modelsCommentTypeComment   = 0
)

var consistencyCheckMap = make(map[string]func(t assert.TestingT, bean interface{}))

// CheckConsistencyFor test that all matching database entries are consistent
func CheckConsistencyFor(t assert.TestingT, beansToCheck ...interface{}) {
	for _, bean := range beansToCheck {
		sliceType := reflect.SliceOf(reflect.TypeOf(bean))
		sliceValue := reflect.MakeSlice(sliceType, 0, 10)

		ptrToSliceValue := reflect.New(sliceType)
		ptrToSliceValue.Elem().Set(sliceValue)

		assert.NoError(t, db.GetEngine(db.DefaultContext).Table(bean).Find(ptrToSliceValue.Interface()))
		sliceValue = ptrToSliceValue.Elem()

		for i := 0; i < sliceValue.Len(); i++ {
			entity := sliceValue.Index(i).Interface()
			checkForConsistency(t, entity)
		}
	}
}

func checkForConsistency(t assert.TestingT, bean interface{}) {
	tb, err := db.TableInfo(bean)
	assert.NoError(t, err)
	f := consistencyCheckMap[tb.Name]
	if f == nil {
		assert.Fail(t, "unknown bean type: %#v", bean)
		return
	}
	f(t, bean)
}

func init() {
	parseBool := func(v string) bool {
		b, _ := strconv.ParseBool(v)
		return b
	}
	parseInt := func(v string) int {
		i, _ := strconv.Atoi(v)
		return i
	}

	checkForUserConsistency := func(t assert.TestingT, bean interface{}) {
		user := reflectionWrap(bean)
		AssertCountByCond(t, "repository", builder.Eq{"owner_id": user.int("ID")}, user.int("NumRepos"))
		AssertCountByCond(t, "star", builder.Eq{"uid": user.int("ID")}, user.int("NumStars"))
		AssertCountByCond(t, "org_user", builder.Eq{"org_id": user.int("ID")}, user.int("NumMembers"))
		AssertCountByCond(t, "team", builder.Eq{"org_id": user.int("ID")}, user.int("NumTeams"))
		AssertCountByCond(t, "follow", builder.Eq{"user_id": user.int("ID")}, user.int("NumFollowing"))
		AssertCountByCond(t, "follow", builder.Eq{"follow_id": user.int("ID")}, user.int("NumFollowers"))
		if user.int("Type") != modelsUserTypeOrganization {
			assert.EqualValues(t, 0, user.int("NumMembers"))
			assert.EqualValues(t, 0, user.int("NumTeams"))
		}
	}

	checkForRepoConsistency := func(t assert.TestingT, bean interface{}) {
		repo := reflectionWrap(bean)
		assert.Equal(t, repo.str("LowerName"), strings.ToLower(repo.str("Name")), "repo: %+v", repo)
		AssertCountByCond(t, "star", builder.Eq{"repo_id": repo.int("ID")}, repo.int("NumStars"))
		AssertCountByCond(t, "milestone", builder.Eq{"repo_id": repo.int("ID")}, repo.int("NumMilestones"))
		AssertCountByCond(t, "repository", builder.Eq{"fork_id": repo.int("ID")}, repo.int("NumForks"))
		if repo.bool("IsFork") {
			AssertExistsAndLoadMap(t, "repository", builder.Eq{"id": repo.int("ForkID")})
		}

		actual := GetCountByCond(t, "watch", builder.Eq{"repo_id": repo.int("ID")}.
			And(builder.Neq{"mode": modelsRepoWatchModeDont}))
		assert.EqualValues(t, repo.int("NumWatches"), actual,
			"Unexpected number of watches for repo %+v", repo)

		actual = GetCountByCond(t, "issue", builder.Eq{"is_pull": false, "repo_id": repo.int("ID")})
		assert.EqualValues(t, repo.int("NumIssues"), actual,
			"Unexpected number of issues for repo %+v", repo)

		actual = GetCountByCond(t, "issue", builder.Eq{"is_pull": false, "is_closed": true, "repo_id": repo.int("ID")})
		assert.EqualValues(t, repo.int("NumClosedIssues"), actual,
			"Unexpected number of closed issues for repo %+v", repo)

		actual = GetCountByCond(t, "issue", builder.Eq{"is_pull": true, "repo_id": repo.int("ID")})
		assert.EqualValues(t, repo.int("NumPulls"), actual,
			"Unexpected number of pulls for repo %+v", repo)

		actual = GetCountByCond(t, "issue", builder.Eq{"is_pull": true, "is_closed": true, "repo_id": repo.int("ID")})
		assert.EqualValues(t, repo.int("NumClosedPulls"), actual,
			"Unexpected number of closed pulls for repo %+v", repo)

		actual = GetCountByCond(t, "milestone", builder.Eq{"is_closed": true, "repo_id": repo.int("ID")})
		assert.EqualValues(t, repo.int("NumClosedMilestones"), actual,
			"Unexpected number of closed milestones for repo %+v", repo)
	}

	checkForIssueConsistency := func(t assert.TestingT, bean interface{}) {
		issue := reflectionWrap(bean)
		typeComment := modelsCommentTypeComment
		actual := GetCountByCond(t, "comment", builder.Eq{"`type`": typeComment, "issue_id": issue.int("ID")})
		assert.EqualValues(t, issue.int("NumComments"), actual, "Unexpected number of comments for issue %+v", issue)
		if issue.bool("IsPull") {
			prRow := AssertExistsAndLoadMap(t, "pull_request", builder.Eq{"issue_id": issue.int("ID")})
			assert.EqualValues(t, parseInt(prRow["index"]), issue.int("Index"))
		}
	}

	checkForPullRequestConsistency := func(t assert.TestingT, bean interface{}) {
		pr := reflectionWrap(bean)
		issueRow := AssertExistsAndLoadMap(t, "issue", builder.Eq{"id": pr.int("IssueID")})
		assert.True(t, parseBool(issueRow["is_pull"]))
		assert.EqualValues(t, parseInt(issueRow["index"]), pr.int("Index"))
	}

	checkForMilestoneConsistency := func(t assert.TestingT, bean interface{}) {
		milestone := reflectionWrap(bean)
		AssertCountByCond(t, "issue", builder.Eq{"milestone_id": milestone.int("ID")}, milestone.int("NumIssues"))

		actual := GetCountByCond(t, "issue", builder.Eq{"is_closed": true, "milestone_id": milestone.int("ID")})
		assert.EqualValues(t, milestone.int("NumClosedIssues"), actual, "Unexpected number of closed issues for milestone %+v", milestone)

		completeness := 0
		if milestone.int("NumIssues") > 0 {
			completeness = milestone.int("NumClosedIssues") * 100 / milestone.int("NumIssues")
		}
		assert.Equal(t, completeness, milestone.int("Completeness"))
	}

	checkForLabelConsistency := func(t assert.TestingT, bean interface{}) {
		label := reflectionWrap(bean)
		issueLabels, err := db.GetEngine(db.DefaultContext).Table("issue_label").
			Where(builder.Eq{"label_id": label.int("ID")}).
			Query()
		assert.NoError(t, err)

		assert.EqualValues(t, label.int("NumIssues"), len(issueLabels), "Unexpected number of issue for label %+v", label)

		issueIDs := make([]int, len(issueLabels))
		for i, issueLabel := range issueLabels {
			issueIDs[i], _ = strconv.Atoi(string(issueLabel["issue_id"]))
		}

		expected := int64(0)
		if len(issueIDs) > 0 {
			expected = GetCountByCond(t, "issue", builder.In("id", issueIDs).And(builder.Eq{"is_closed": true}))
		}
		assert.EqualValues(t, expected, label.int("NumClosedIssues"), "Unexpected number of closed issues for label %+v", label)
	}

	checkForTeamConsistency := func(t assert.TestingT, bean interface{}) {
		team := reflectionWrap(bean)
		AssertCountByCond(t, "team_user", builder.Eq{"team_id": team.int("ID")}, team.int("NumMembers"))
		AssertCountByCond(t, "team_repo", builder.Eq{"team_id": team.int("ID")}, team.int("NumRepos"))
	}

	checkForActionConsistency := func(t assert.TestingT, bean interface{}) {
		action := reflectionWrap(bean)
		repoRow := AssertExistsAndLoadMap(t, "repository", builder.Eq{"id": action.int("RepoID")})
		assert.Equal(t, parseBool(repoRow["is_private"]), action.bool("IsPrivate"), "action: %+v", action)
	}

	consistencyCheckMap["user"] = checkForUserConsistency
	consistencyCheckMap["repository"] = checkForRepoConsistency
	consistencyCheckMap["issue"] = checkForIssueConsistency
	consistencyCheckMap["pull_request"] = checkForPullRequestConsistency
	consistencyCheckMap["milestone"] = checkForMilestoneConsistency
	consistencyCheckMap["label"] = checkForLabelConsistency
	consistencyCheckMap["team"] = checkForTeamConsistency
	consistencyCheckMap["action"] = checkForActionConsistency
}
