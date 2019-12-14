// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "xorm.io/xorm"

// InsertMilestones creates milestones of repository.
func InsertMilestones(ms ...*Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	// to return the id, so we should not use batch insert
	for _, m := range ms {
		if _, err = sess.NoAutoTime().Insert(m); err != nil {
			return err
		}
	}

	if _, err = sess.Exec("UPDATE `repository` SET num_milestones = num_milestones + ? WHERE id = ?", len(ms), ms[0].RepoID); err != nil {
		return err
	}
	return sess.Commit()
}

// InsertIssues insert issues to database
func InsertIssues(issues ...*Issue) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	for _, issue := range issues {
		if err := insertIssue(sess, issue); err != nil {
			return err
		}
	}
	return sess.Commit()
}

func insertIssue(sess *xorm.Session, issue *Issue) error {
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	var issueLabels = make([]IssueLabel, 0, len(issue.Labels))
	var labelIDs = make([]int64, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		issueLabels = append(issueLabels, IssueLabel{
			IssueID: issue.ID,
			LabelID: label.ID,
		})
		labelIDs = append(labelIDs, label.ID)
	}
	if _, err := sess.Insert(issueLabels); err != nil {
		return err
	}

	cols := make([]string, 0)
	if !issue.IsPull {
		sess.ID(issue.RepoID).Incr("num_issues")
		cols = append(cols, "num_issues")
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
			cols = append(cols, "num_closed_issues")
		}
	} else {
		sess.ID(issue.RepoID).Incr("num_pulls")
		cols = append(cols, "num_pulls")
		if issue.IsClosed {
			sess.Incr("num_closed_pulls")
			cols = append(cols, "num_closed_pulls")
		}
	}
	if _, err := sess.NoAutoTime().Cols(cols...).Update(issue.Repo); err != nil {
		return err
	}

	cols = []string{"num_issues"}
	sess.Incr("num_issues")
	if issue.IsClosed {
		sess.Incr("num_closed_issues")
		cols = append(cols, "num_closed_issues")
	}
	if _, err := sess.In("id", labelIDs).NoAutoTime().Cols(cols...).Update(new(Label)); err != nil {
		return err
	}

	if issue.MilestoneID > 0 {
		cols = []string{"num_issues"}
		sess.Incr("num_issues")
		cl := "num_closed_issues"
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
			cols = append(cols, "num_closed_issues")
			cl = "(num_closed_issues + 1)"
		}

		if _, err := sess.ID(issue.MilestoneID).
			SetExpr("completeness", cl+" * 100 / (num_issues + 1)").
			NoAutoTime().Cols(cols...).
			Update(new(Milestone)); err != nil {
			return err
		}
	}

	return nil
}

// InsertIssueComments inserts many comments of issues.
func InsertIssueComments(comments []*Comment) error {
	if len(comments) == 0 {
		return nil
	}

	var issueIDs = make(map[int64]bool)
	for _, comment := range comments {
		issueIDs[comment.IssueID] = true
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.NoAutoTime().Insert(comments); err != nil {
		return err
	}
	for issueID := range issueIDs {
		if _, err := sess.Exec("UPDATE issue set num_comments = (SELECT count(*) FROM comment WHERE issue_id = ?) WHERE id = ?", issueID, issueID); err != nil {
			return err
		}
	}
	return sess.Commit()
}

// InsertPullRequests inserted pull requests
func InsertPullRequests(prs ...*PullRequest) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	for _, pr := range prs {
		if err := insertIssue(sess, pr.Issue); err != nil {
			return err
		}
		pr.IssueID = pr.Issue.ID
		if _, err := sess.NoAutoTime().Insert(pr); err != nil {
			return err
		}
	}

	return sess.Commit()
}

// InsertReleases migrates release
func InsertReleases(rels ...*Release) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	for _, rel := range rels {
		if _, err := sess.NoAutoTime().Insert(rel); err != nil {
			return err
		}

		for i := 0; i < len(rel.Attachments); i++ {
			rel.Attachments[i].ReleaseID = rel.ID
		}

		if _, err := sess.NoAutoTime().Insert(rel.Attachments); err != nil {
			return err
		}
	}

	return sess.Commit()
}
