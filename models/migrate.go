// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "github.com/go-xorm/xorm"

// InsertIssue insert one issue to database
func InsertIssue(issue *Issue, labelIDs []int64) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := insertIssue(sess, issue, labelIDs); err != nil {
		return err
	}
	return sess.Commit()
}

func insertIssue(sess *xorm.Session, issue *Issue, labelIDs []int64) error {
	if issue.MilestoneID > 0 {
		sess.Incr("num_issues")
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
		}
		if _, err := sess.ID(issue.MilestoneID).NoAutoTime().Update(new(Milestone)); err != nil {
			return err
		}
	}
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	var issueLabels = make([]IssueLabel, 0, len(labelIDs))
	for _, labelID := range labelIDs {
		issueLabels = append(issueLabels, IssueLabel{
			IssueID: issue.ID,
			LabelID: labelID,
		})
	}
	if _, err := sess.Insert(issueLabels); err != nil {
		return err
	}
	if !issue.IsPull {
		sess.ID(issue.RepoID).Incr("num_issues")
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
		}
	} else {
		sess.ID(issue.RepoID).Incr("num_pulls")
		if issue.IsClosed {
			sess.Incr("num_closed_pulls")
		}
	}
	if _, err := sess.NoAutoTime().Update(issue.Repo); err != nil {
		return err
	}

	sess.Incr("num_issues")
	if issue.IsClosed {
		sess.Incr("num_closed_issues")
	}
	if _, err := sess.In("id", labelIDs).Update(new(Label)); err != nil {
		return err
	}

	if issue.MilestoneID > 0 {
		if _, err := sess.ID(issue.MilestoneID).SetExpr("completeness", "num_closed_issues * 100 / num_issues").Update(new(Milestone)); err != nil {
			return err
		}
	}

	return nil
}

// InsertComment inserted a comment
func InsertComment(comment *Comment) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.NoAutoTime().Insert(comment); err != nil {
		return err
	}
	if _, err := sess.ID(comment.IssueID).Incr("num_comments").Update(new(Issue)); err != nil {
		return err
	}
	return sess.Commit()
}

// InsertPullRequest inserted a pull request
func InsertPullRequest(pr *PullRequest, labelIDs []int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := insertIssue(sess, pr.Issue, labelIDs); err != nil {
		return err
	}
	pr.IssueID = pr.Issue.ID
	if _, err := sess.NoAutoTime().Insert(pr); err != nil {
		return err
	}
	return sess.Commit()
}

// MigrateRelease migrates release
func MigrateRelease(rel *Release) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	var oriRel = Release{
		RepoID:  rel.RepoID,
		TagName: rel.TagName,
	}
	exist, err := sess.Get(&oriRel)
	if err != nil {
		return err
	}
	if !exist {
		if _, err := sess.NoAutoTime().Insert(rel); err != nil {
			return err
		}
	} else {
		rel.ID = oriRel.ID
		if _, err := sess.ID(rel.ID).Cols("target, title, note, is_tag, num_commits").Update(rel); err != nil {
			return err
		}
	}

	for i := 0; i < len(rel.Attachments); i++ {
		rel.Attachments[i].ReleaseID = rel.ID
	}

	if _, err := sess.NoAutoTime().Insert(rel.Attachments); err != nil {
		return err
	}

	return sess.Commit()
}
