// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"reflect"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/unittestbridge"

	"xorm.io/builder"
)

// CheckConsistencyFor test that all matching database entries are consistent
func CheckConsistencyFor(t unittestbridge.Tester, beansToCheck ...interface{}) {
	ta := unittestbridge.NewAsserter(t)
	for _, bean := range beansToCheck {
		sliceType := reflect.SliceOf(reflect.TypeOf(bean))
		sliceValue := reflect.MakeSlice(sliceType, 0, 10)

		ptrToSliceValue := reflect.New(sliceType)
		ptrToSliceValue.Elem().Set(sliceValue)

		ta.NoError(db.GetEngine(db.DefaultContext).Table(bean).Find(ptrToSliceValue.Interface()))
		sliceValue = ptrToSliceValue.Elem()

		for i := 0; i < sliceValue.Len(); i++ {
			entity := sliceValue.Index(i).Interface()
			checkForConsistency(ta, entity)
		}
	}
}

func checkForConsistency(ta unittestbridge.Asserter, bean interface{}) {
	switch b := bean.(type) {
	case *User:
		checkForUserConsistency(b, ta)
	case *Repository:
		checkForRepoConsistency(b, ta)
	case *Issue:
		checkForIssueConsistency(b, ta)
	case *PullRequest:
		checkForPullRequestConsistency(b, ta)
	case *Milestone:
		checkForMilestoneConsistency(b, ta)
	case *Label:
		checkForLabelConsistency(b, ta)
	case *Team:
		checkForTeamConsistency(b, ta)
	case *Action:
		checkForActionConsistency(b, ta)
	default:
		ta.Errorf("unknown bean type: %#v", bean)
	}
}

// getCount get the count of database entries matching bean
func getCount(ta unittestbridge.Asserter, e db.Engine, bean interface{}) int64 {
	count, err := e.Count(bean)
	ta.NoError(err)
	return count
}

// assertCount test the count of database entries matching bean
func assertCount(ta unittestbridge.Asserter, bean interface{}, expected int) {
	ta.EqualValues(expected, getCount(ta, db.GetEngine(db.DefaultContext), bean),
		"Failed consistency test, the counted bean (of type %T) was %+v", bean, bean)
}

func checkForUserConsistency(user *User, ta unittestbridge.Asserter) {
	assertCount(ta, &Repository{OwnerID: user.ID}, user.NumRepos)
	assertCount(ta, &Star{UID: user.ID}, user.NumStars)
	assertCount(ta, &OrgUser{OrgID: user.ID}, user.NumMembers)
	assertCount(ta, &Team{OrgID: user.ID}, user.NumTeams)
	assertCount(ta, &Follow{UserID: user.ID}, user.NumFollowing)
	assertCount(ta, &Follow{FollowID: user.ID}, user.NumFollowers)
	if user.Type != UserTypeOrganization {
		ta.EqualValues(0, user.NumMembers)
		ta.EqualValues(0, user.NumTeams)
	}
}

func checkForRepoConsistency(repo *Repository, ta unittestbridge.Asserter) {
	ta.Equal(repo.LowerName, strings.ToLower(repo.Name), "repo: %+v", repo)
	assertCount(ta, &Star{RepoID: repo.ID}, repo.NumStars)
	assertCount(ta, &Milestone{RepoID: repo.ID}, repo.NumMilestones)
	assertCount(ta, &Repository{ForkID: repo.ID}, repo.NumForks)
	if repo.IsFork {
		db.AssertExistsAndLoadBean(ta, &Repository{ID: repo.ForkID})
	}

	actual := getCount(ta, db.GetEngine(db.DefaultContext).Where("Mode<>?", RepoWatchModeDont), &Watch{RepoID: repo.ID})
	ta.EqualValues(repo.NumWatches, actual,
		"Unexpected number of watches for repo %+v", repo)

	actual = getCount(ta, db.GetEngine(db.DefaultContext).Where("is_pull=?", false), &Issue{RepoID: repo.ID})
	ta.EqualValues(repo.NumIssues, actual,
		"Unexpected number of issues for repo %+v", repo)

	actual = getCount(ta, db.GetEngine(db.DefaultContext).Where("is_pull=? AND is_closed=?", false, true), &Issue{RepoID: repo.ID})
	ta.EqualValues(repo.NumClosedIssues, actual,
		"Unexpected number of closed issues for repo %+v", repo)

	actual = getCount(ta, db.GetEngine(db.DefaultContext).Where("is_pull=?", true), &Issue{RepoID: repo.ID})
	ta.EqualValues(repo.NumPulls, actual,
		"Unexpected number of pulls for repo %+v", repo)

	actual = getCount(ta, db.GetEngine(db.DefaultContext).Where("is_pull=? AND is_closed=?", true, true), &Issue{RepoID: repo.ID})
	ta.EqualValues(repo.NumClosedPulls, actual,
		"Unexpected number of closed pulls for repo %+v", repo)

	actual = getCount(ta, db.GetEngine(db.DefaultContext).Where("is_closed=?", true), &Milestone{RepoID: repo.ID})
	ta.EqualValues(repo.NumClosedMilestones, actual,
		"Unexpected number of closed milestones for repo %+v", repo)
}

func checkForIssueConsistency(issue *Issue, ta unittestbridge.Asserter) {
	actual := getCount(ta, db.GetEngine(db.DefaultContext).Where("type=?", CommentTypeComment), &Comment{IssueID: issue.ID})
	ta.EqualValues(issue.NumComments, actual,
		"Unexpected number of comments for issue %+v", issue)
	if issue.IsPull {
		pr := db.AssertExistsAndLoadBean(ta, &PullRequest{IssueID: issue.ID}).(*PullRequest)
		ta.EqualValues(pr.Index, issue.Index)
	}
}

func checkForPullRequestConsistency(pr *PullRequest, ta unittestbridge.Asserter) {
	issue := db.AssertExistsAndLoadBean(ta, &Issue{ID: pr.IssueID}).(*Issue)
	ta.True(issue.IsPull)
	ta.EqualValues(issue.Index, pr.Index)
}

func checkForMilestoneConsistency(milestone *Milestone, ta unittestbridge.Asserter) {
	assertCount(ta, &Issue{MilestoneID: milestone.ID}, milestone.NumIssues)

	actual := getCount(ta, db.GetEngine(db.DefaultContext).Where("is_closed=?", true), &Issue{MilestoneID: milestone.ID})
	ta.EqualValues(milestone.NumClosedIssues, actual,
		"Unexpected number of closed issues for milestone %+v", milestone)

	completeness := 0
	if milestone.NumIssues > 0 {
		completeness = milestone.NumClosedIssues * 100 / milestone.NumIssues
	}
	ta.Equal(completeness, milestone.Completeness)
}

func checkForLabelConsistency(label *Label, ta unittestbridge.Asserter) {
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
		expected = getCount(ta, db.GetEngine(db.DefaultContext).In("id", issueIDs).Where("is_closed=?", true), &Issue{})
	}
	ta.EqualValues(expected, label.NumClosedIssues,
		"Unexpected number of closed issues for label %+v", label)
}

func checkForTeamConsistency(team *Team, ta unittestbridge.Asserter) {
	assertCount(ta, &TeamUser{TeamID: team.ID}, team.NumMembers)
	assertCount(ta, &TeamRepo{TeamID: team.ID}, team.NumRepos)
}

func checkForActionConsistency(action *Action, ta unittestbridge.Asserter) {
	repo := db.AssertExistsAndLoadBean(ta, &Repository{ID: action.RepoID}).(*Repository)
	ta.Equal(repo.IsPrivate, action.IsPrivate, "action: %+v", action)
}

// CountOrphanedLabels return count of labels witch are broken and not accessible via ui anymore
func CountOrphanedLabels() (int64, error) {
	noref, err := db.GetEngine(db.DefaultContext).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Count("label.id")
	if err != nil {
		return 0, err
	}

	norepo, err := db.GetEngine(db.DefaultContext).Table("label").
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	noorg, err := db.GetEngine(db.DefaultContext).Table("label").
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	return noref + norepo + noorg, nil
}

// DeleteOrphanedLabels delete labels witch are broken and not accessible via ui anymore
func DeleteOrphanedLabels() error {
	// delete labels with no reference
	if _, err := db.GetEngine(db.DefaultContext).Table("label").Where("repo_id=? AND org_id=?", 0, 0).Delete(new(Label)); err != nil {
		return err
	}

	// delete labels with none existing repos
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	// delete labels with none existing orgs
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Delete(Label{}); err != nil {
		return err
	}

	return nil
}

// CountOrphanedIssueLabels return count of IssueLabels witch have no label behind anymore
func CountOrphanedIssueLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("issue_label").
		NotIn("label_id", builder.Select("id").From("label")).
		Count()
}

// DeleteOrphanedIssueLabels delete IssueLabels witch have no label behind anymore
func DeleteOrphanedIssueLabels() error {
	_, err := db.GetEngine(db.DefaultContext).
		NotIn("label_id", builder.Select("id").From("label")).
		Delete(IssueLabel{})

	return err
}

// CountOrphanedIssues count issues without a repo
func CountOrphanedIssues() (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("issue").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).
		Count("id")
}

// DeleteOrphanedIssues delete issues without a repo
func DeleteOrphanedIssues() error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	var ids []int64

	if err := db.GetEngine(ctx).Table("issue").Distinct("issue.repo_id").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).GroupBy("issue.repo_id").
		Find(&ids); err != nil {
		return err
	}

	var attachmentPaths []string
	for i := range ids {
		paths, err := deleteIssuesByRepoID(db.GetEngine(ctx), ids[i])
		if err != nil {
			return err
		}
		attachmentPaths = append(attachmentPaths, paths...)
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	// Remove issue attachment files.
	for i := range attachmentPaths {
		removeAllWithNotice(db.GetEngine(db.DefaultContext), "Delete issue attachment", attachmentPaths[i])
	}
	return nil
}

// CountOrphanedObjects count subjects with have no existing refobject anymore
func CountOrphanedObjects(subject, refobject, joinCond string) (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"}).
		Count("id")
}

// DeleteOrphanedObjects delete subjects with have no existing refobject anymore
func DeleteOrphanedObjects(subject, refobject, joinCond string) error {
	subQuery := builder.Select("`"+subject+"`.id").
		From("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"})
	sql, args, err := builder.Delete(builder.In("id", subQuery)).From("`" + subject + "`").ToSQL()
	if err != nil {
		return err
	}
	_, err = db.GetEngine(db.DefaultContext).Exec(append([]interface{}{sql}, args...)...)
	return err
}

// CountNullArchivedRepository counts the number of repositories with is_archived is null
func CountNullArchivedRepository() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.IsNull{"is_archived"}).Count(new(Repository))
}

// FixNullArchivedRepository sets is_archived to false where it is null
func FixNullArchivedRepository() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.IsNull{"is_archived"}).Cols("is_archived").NoAutoTime().Update(&Repository{
		IsArchived: false,
	})
}

// CountWrongUserType count OrgUser who have wrong type
func CountWrongUserType() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": 0}.And(builder.Neq{"num_teams": 0})).Count(new(User))
}

// FixWrongUserType fix OrgUser who have wrong type
func FixWrongUserType() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": 0}.And(builder.Neq{"num_teams": 0})).Cols("type").NoAutoTime().Update(&User{Type: 1})
}

// CountCommentTypeLabelWithEmptyLabel count label comments with empty label
func CountCommentTypeLabelWithEmptyLabel() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Count(new(Comment))
}

// FixCommentTypeLabelWithEmptyLabel count label comments with empty label
func FixCommentTypeLabelWithEmptyLabel() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Delete(new(Comment))
}

// CountCommentTypeLabelWithOutsideLabels count label comments with outside label
func CountCommentTypeLabelWithOutsideLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("comment.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))", CommentTypeLabel).
		Table("comment").
		Join("inner", "label", "label.id = comment.label_id").
		Join("inner", "issue", "issue.id = comment.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count(new(Comment))
}

// FixCommentTypeLabelWithOutsideLabels count label comments with outside label
func FixCommentTypeLabelWithOutsideLabels() (int64, error) {
	res, err := db.GetEngine(db.DefaultContext).Exec(`DELETE FROM comment WHERE comment.id IN (
		SELECT il_too.id FROM (
			SELECT com.id
				FROM comment AS com
					INNER JOIN label ON com.label_id = label.id
					INNER JOIN issue on issue.id = com.issue_id
					INNER JOIN repository ON issue.repo_id = repository.id
				WHERE
					com.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))
	) AS il_too)`, CommentTypeLabel)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// CountIssueLabelWithOutsideLabels count label comments with outside label
func CountIssueLabelWithOutsideLabels() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(builder.Expr("(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)")).
		Table("issue_label").
		Join("inner", "label", "issue_label.label_id = label.id ").
		Join("inner", "issue", "issue.id = issue_label.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count(new(IssueLabel))
}

// FixIssueLabelWithOutsideLabels fix label comments with outside label
func FixIssueLabelWithOutsideLabels() (int64, error) {
	res, err := db.GetEngine(db.DefaultContext).Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
		SELECT il_too.id FROM (
			SELECT il_too_too.id
				FROM issue_label AS il_too_too
					INNER JOIN label ON il_too_too.label_id = label.id
					INNER JOIN issue on issue.id = il_too_too.issue_id
					INNER JOIN repository on repository.id = issue.repo_id
				WHERE
					(label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id)
	) AS il_too )`)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
